package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type SupabaseStorageConnector struct {
	name           string
	supabaseURL    string
	serviceRoleKey string
	bucketName     string
	prefix         string
	client         *http.Client
}

func NewSupabaseStorageConnector(name string, config map[string]interface{}) (*SupabaseStorageConnector, error) {
	supabaseURL, _ := config["supabase_url"].(string)
	serviceKey, _ := config["service_role_key"].(string)
	if serviceKey == "" {
		serviceKey, _ = config["api_key"].(string)
	}
	bucketName, _ := config["bucket_name"].(string)
	if bucketName == "" {
		bucketName, _ = config["bucket"].(string)
	}
	prefix, _ := config["prefix"].(string)

	if supabaseURL == "" || bucketName == "" {
		return nil, fmt.Errorf("supabase storage requires 'supabase_url' and 'bucket_name'")
	}

	return &SupabaseStorageConnector{
		name:           name,
		supabaseURL:    strings.TrimRight(supabaseURL, "/"),
		serviceRoleKey: serviceKey,
		bucketName:     bucketName,
		prefix:         prefix,
		client:         &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *SupabaseStorageConnector) Type() string { return "supabase_storage" }
func (s *SupabaseStorageConnector) Name() string { return s.name }

func (s *SupabaseStorageConnector) ValidateConfig(ctx context.Context) error {
	if s.supabaseURL == "" || s.bucketName == "" {
		return fmt.Errorf("supabase_url and bucket_name are required")
	}
	return nil
}

type supabaseFileObject struct {
	Name           string                 `json:"name"`
	ID             string                 `json:"id"`
	UpdatedAt      string                 `json:"updated_at"`
	CreatedAt      string                 `json:"created_at"`
	LastAccessedAt string                 `json:"last_accessed_at"`
	Metadata       map[string]interface{} `json:"metadata"`
}

func (s *SupabaseStorageConnector) ListObjects(ctx context.Context, prefix string) ([]*StorageObject, error) {
	effectivePrefix := s.prefix
	if prefix != "" {
		if effectivePrefix != "" {
			effectivePrefix = effectivePrefix + "/" + prefix
		} else {
			effectivePrefix = prefix
		}
	}

	url := fmt.Sprintf("%s/storage/v1/object/list/%s", s.supabaseURL, s.bucketName)
	payload := map[string]interface{}{
		"prefix": effectivePrefix,
		"limit":  1000,
		"sortBy": map[string]string{
			"column": "name",
			"order":  "asc",
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if s.serviceRoleKey != "" {
		req.Header.Set("apikey", s.serviceRoleKey)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.serviceRoleKey))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list supabase objects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("supabase list objects returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var files []supabaseFileObject
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode supabase response: %w", err)
	}

	var objects []*StorageObject
	for _, f := range files {
		// Ignore folder placeholders (ID is null or name has no dot or metadata size is 0)
		size := int64(0)
		mimetype := "application/octet-stream"
		etag := f.ID
		if f.Metadata != nil {
			if sz, ok := f.Metadata["size"].(float64); ok {
				size = int64(sz)
			}
			if mt, ok := f.Metadata["mimetype"].(string); ok {
				mimetype = mt
			}
			if et, ok := f.Metadata["eTag"].(string); ok {
				etag = strings.Trim(et, "\"")
			}
		}

		key := f.Name
		if effectivePrefix != "" {
			key = fmt.Sprintf("%s/%s", effectivePrefix, f.Name)
		}

		updatedTime, _ := time.Parse(time.RFC3339, f.UpdatedAt)

		objects = append(objects, &StorageObject{
			Key:          key,
			ETag:         etag,
			Size:         size,
			LastModified: updatedTime,
			ContentType:  mimetype,
			Metadata: map[string]interface{}{
				"source": "supabase_storage",
				"bucket": s.bucketName,
				"id":     f.ID,
			},
		})
	}

	return objects, nil
}

func (s *SupabaseStorageConnector) FetchObject(ctx context.Context, key string) ([]byte, *StorageObject, error) {
	url := fmt.Sprintf("%s/storage/v1/object/authenticated/%s/%s", s.supabaseURL, s.bucketName, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, err
	}

	if s.serviceRoleKey != "" {
		req.Header.Set("apikey", s.serviceRoleKey)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.serviceRoleKey))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch supabase object: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("supabase get object returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read supabase object body: %w", err)
	}

	etag := strings.Trim(resp.Header.Get("ETag"), "\"")
	lastMod, _ := http.ParseTime(resp.Header.Get("Last-Modified"))

	obj := &StorageObject{
		Key:          key,
		ETag:         etag,
		Size:         int64(len(data)),
		LastModified: lastMod,
		ContentType:  resp.Header.Get("Content-Type"),
		Metadata: map[string]interface{}{
			"source": "supabase_storage",
			"bucket": s.bucketName,
		},
	}

	return data, obj, nil
}
