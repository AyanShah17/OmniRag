// Package middleware provides HTTP middleware for the Go ingestion engine,
// including strict Clerk JWT authentication for production deployments.
package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// contextKey avoids collisions with other packages' context keys.
type contextKey string

const (
	userIDContextKey    contextKey = "clerk_user_id"
	tenantIDContextKey  contextKey = "clerk_tenant_id"
	workspaceContextKey contextKey = "clerk_workspace_id"
	rolesContextKey     contextKey = "clerk_roles"
)

// AuthConfig controls how ClerkAuthMiddleware behaves.
type AuthConfig struct {
	// Mode is "development" or "production". In production, every request
	// (aside from explicitly exempted paths) must carry a valid Clerk JWT or
	// the request is rejected with 401. In development, an unauthenticated
	// request is allowed through with a fixed local dev identity, mirroring
	// the Python service's dev fallback so local end-to-end flows keep working.
	Mode string

	// JWKSURL is Clerk's JSON Web Key Set endpoint used to verify RS256
	// signatures. Required when Mode is "production".
	JWKSURL string

	// Issuer, when set, is checked against the token's `iss` claim.
	Issuer string

	// ExemptPaths are path prefixes that bypass auth entirely, such as health checks.
	ExemptPaths []string
}

func (c AuthConfig) isProduction() bool {
	return strings.EqualFold(c.Mode, "production")
}

// ClerkClaims is the subset of Clerk's JWT claims this middleware relies on.
type ClerkClaims struct {
	jwt.RegisteredClaims
	OrgID       string `json:"org_id"`
	OrgRole     string `json:"org_role"`
	WorkspaceID string `json:"workspace_id"`
}

// jwks caches Clerk's public signing keys so we don't fetch them on every request.
type jwks struct {
	mu        sync.RWMutex
	url       string
	keys      map[string]*rsa.PublicKey
	fetchedAt time.Time
	ttl       time.Duration
	client    *http.Client
}

func newJWKS(url string) *jwks {
	return &jwks{
		url:    url,
		keys:   make(map[string]*rsa.PublicKey),
		ttl:    10 * time.Minute,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type jwkRaw struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwkRaw `json:"keys"`
}

func (j *jwks) refreshIfNeeded(ctx context.Context) error {
	j.mu.RLock()
	stale := time.Since(j.fetchedAt) > j.ttl || len(j.keys) == 0
	j.mu.RUnlock()
	if !stale {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		return err
	}
	resp, err := j.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("JWKS endpoint returned status %d", resp.StatusCode)
	}

	var parsed jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decoding JWKS response: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(parsed.Keys))
	for _, k := range parsed.Keys {
		if k.Kty != "RSA" || k.Kid == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(k.N, k.E)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	j.mu.Lock()
	j.keys = keys
	j.fetchedAt = time.Now()
	j.mu.Unlock()
	return nil
}

func (j *jwks) getKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if err := j.refreshIfNeeded(ctx); err != nil {
		return nil, err
	}
	j.mu.RLock()
	key, ok := j.keys[kid]
	j.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no matching signing key for kid=%s", kid)
	}
	return key, nil
}

func rsaPublicKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eB64)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid exponent in JWK")
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// ClerkAuthMiddleware verifies Clerk-issued JWTs and attaches the resolved
// identity to the request context. In production mode, requests without a
// valid token are rejected with 401 — there is no fallback identity.
func ClerkAuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	var keySet *jwks
	if cfg.JWKSURL != "" {
		keySet = newJWKS(cfg.JWKSURL)
	}

	if cfg.isProduction() && keySet == nil {
		// Fail loudly at startup rather than silently letting every request
		// through unauthenticated.
		panic("middleware: AUTH_MODE=production requires CLERK_JWKS_URL to be set")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, prefix := range cfg.ExemptPaths {
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			token := extractBearerToken(r)

			if token == "" {
				if cfg.isProduction() {
					writeUnauthorized(w, "missing bearer token")
					return
				}
				// Development fallback identity, mirroring the Python service.
				ctx := withIdentity(r.Context(), "user_dev_enterprise", "tenant_default", devWorkspaceID(r), []string{"admin"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			claims, err := verifyToken(r.Context(), token, keySet, cfg.Issuer)
			if err != nil {
				if cfg.isProduction() {
					writeUnauthorized(w, "invalid or expired token")
					return
				}
				// In dev mode a bad token still degrades gracefully to the dev identity
				// rather than blocking local iteration.
				ctx := withIdentity(r.Context(), "user_dev_enterprise", "tenant_default", devWorkspaceID(r), []string{"admin"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			if claims.Subject == "" || claims.OrgID == "" {
				writeUnauthorized(w, "token missing required claims")
				return
			}

			workspaceID := claims.WorkspaceID
			if workspaceID == "" {
				workspaceID = r.Header.Get("X-Workspace-ID")
			}
			role := claims.OrgRole
			if role == "" {
				role = "member"
			}

			ctx := withIdentity(r.Context(), claims.Subject, claims.OrgID, workspaceID, []string{role})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func devWorkspaceID(r *http.Request) string {
	if ws := r.Header.Get("X-Workspace-ID"); ws != "" {
		return ws
	}
	return "ws_default"
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func verifyToken(ctx context.Context, tokenStr string, keySet *jwks, issuer string) (*ClerkClaims, error) {
	if keySet == nil {
		return nil, errors.New("authentication provider not configured")
	}

	claims := &ClerkClaims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))

	_, err := parser.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token header missing kid")
		}
		return keySet.getKey(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("token verification failed: %w", err)
	}

	if issuer != "" && claims.Issuer != issuer {
		return nil, fmt.Errorf("unexpected token issuer")
	}

	return claims, nil
}

func writeUnauthorized(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized: " + reason})
}

func withIdentity(ctx context.Context, userID, tenantID, workspaceID string, roles []string) context.Context {
	ctx = context.WithValue(ctx, userIDContextKey, userID)
	ctx = context.WithValue(ctx, tenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, workspaceContextKey, workspaceID)
	ctx = context.WithValue(ctx, rolesContextKey, roles)
	return ctx
}

// UserIDFromContext returns the authenticated user ID, if any.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDContextKey).(string)
	return v, ok
}

// TenantIDFromContext returns the authenticated tenant/org ID, if any.
func TenantIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(tenantIDContextKey).(string)
	return v, ok
}

// WorkspaceIDFromContext returns the resolved workspace ID, if any.
func WorkspaceIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(workspaceContextKey).(string)
	return v, ok
}

// RolesFromContext returns the authenticated user's roles, if any.
func RolesFromContext(ctx context.Context) ([]string, bool) {
	v, ok := ctx.Value(rolesContextKey).([]string)
	return v, ok
}
