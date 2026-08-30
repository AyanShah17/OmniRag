package connectors

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ConfluenceConnector struct {
	name       string
	domain     string
	email      string
	apiToken   string
	spaceKey   string
	cqlQuery   string
	client     *http.Client
}

func NewConfluenceConnector(name string, config map[string]interface{}) (*ConfluenceConnector, error) {
	domain, _ := config["domain"].(string)
	if domain == "" {
		domain, _ = config["base_url"].(string)
	}
	email, _ := config["email"].(string)
	apiToken, _ := config["api_token"].(string)
	spaceKey, _ := config["space_key"].(string)
	cqlQuery, _ := config["cql_query"].(string)

	if domain == "" || email == "" || apiToken == "" {
		return nil, fmt.Errorf("confluence connector requires 'domain', 'email', and 'api_token'")
	}

	return &ConfluenceConnector{
		name:     name,
		domain:   strings.TrimRight(domain, "/"),
		email:    email,
		apiToken: apiToken,
		spaceKey: spaceKey,
		cqlQuery: cqlQuery,
		client:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (c *ConfluenceConnector) Type() string { return "confluence" }
func (c *ConfluenceConnector) Name() string { return c.name }

func (c *ConfluenceConnector) ValidateConfig(ctx context.Context) error {
	if c.domain == "" || c.email == "" || c.apiToken == "" {
		return fmt.Errorf("domain, email, and api_token are required")
	}
	return nil
}

type confluenceContentResponse struct {
	Results []struct {
		ID      string `json:"id"`
		Type    string `json:"type"`
		Status  string `json:"status"`
		Title   string `json:"title"`
		Version struct {
			Number    int    `json:"number"`
			When      string `json:"when"`
			Message   string `json:"message"`
		} `json:"version"`
		Body struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	} `json:"results"`
}

func (c *ConfluenceConnector) ListObjects(ctx context.Context, prefix string) ([]*StorageObject, error) {
	cql := "type=page"
	if c.spaceKey != "" {
		cql = fmt.Sprintf("space='%s' AND type=page", c.spaceKey)
	}
	if c.cqlQuery != "" {
		cql = c.cqlQuery
	}

	reqURL := fmt.Sprintf("%s/wiki/rest/api/content/search?cql=%s&limit=100&expand=version,_links", c.domain, url.QueryEscape(cql))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.email, c.apiToken)))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search confluence: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("confluence search returned status %d: %s", resp.StatusCode, string(body))
	}

	var res confluenceContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to parse confluence json: %w", err)
	}

	var objects []*StorageObject
	for _, page := range res.Results {
		lastMod, _ := time.Parse(time.RFC3339, page.Version.When)
		key := fmt.Sprintf("pages/%s_%s.html", page.ID, sanitizeTitle(page.Title))

		objects = append(objects, &StorageObject{
			Key:          key,
			ETag:         fmt.Sprintf("v%d", page.Version.Number),
			Size:         0,
			LastModified: lastMod,
			ContentType:  "text/html",
			Metadata: map[string]interface{}{
				"source":         "confluence",
				"page_id":        page.ID,
				"title":          page.Title,
				"version_number": page.Version.Number,
				"web_url":        fmt.Sprintf("%s/wiki%s", c.domain, page.Links.WebUI),
			},
		})
	}

	return objects, nil
}

func (c *ConfluenceConnector) FetchObject(ctx context.Context, key string) ([]byte, *StorageObject, error) {
	// Extract page ID from key (e.g. pages/12345_Title.html)
	parts := strings.Split(key, "_")
	pageID := strings.TrimPrefix(parts[0], "pages/")

	reqURL := fmt.Sprintf("%s/wiki/rest/api/content/%s?expand=body.storage,version,_links", c.domain, pageID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, nil, err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", c.email, c.apiToken)))
	req.Header.Set("Authorization", fmt.Sprintf("Basic %s", auth))
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch confluence page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("confluence get page returned status %d: %s", resp.StatusCode, string(body))
	}

	var page struct {
		ID      string `json:"id"`
		Title   string `json:"title"`
		Version struct {
			Number int    `json:"number"`
			When   string `json:"when"`
		} `json:"version"`
		Body struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, nil, fmt.Errorf("failed to decode confluence page body: %w", err)
	}

	htmlContent := page.Body.Storage.Value
	data := []byte(htmlContent)
	lastMod, _ := time.Parse(time.RFC3339, page.Version.When)

	obj := &StorageObject{
		Key:          key,
		ETag:         fmt.Sprintf("v%d", page.Version.Number),
		Size:         int64(len(data)),
		LastModified: lastMod,
		ContentType:  "text/html",
		Metadata: map[string]interface{}{
			"source":         "confluence",
			"page_id":        page.ID,
			"title":          page.Title,
			"version_number": page.Version.Number,
			"web_url":        fmt.Sprintf("%s/wiki%s", c.domain, page.Links.WebUI),
		},
	}

	return data, obj, nil
}

func sanitizeTitle(title string) string {
	var sb strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else if r == ' ' {
			sb.WriteRune('_')
		}
	}
	return sb.String()
}
