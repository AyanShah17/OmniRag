package connectors

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type AzureBlobConnector struct {
	name             string
	accountName      string
	accountKey       string
	containerName    string
	prefix           string
	connectionString string
	client           *http.Client
}

func NewAzureBlobConnector(name string, config map[string]interface{}) (*AzureBlobConnector, error) {
	containerName, _ := config["container_name"].(string)
	if containerName == "" {
		containerName, _ = config["container"].(string)
	}
	if containerName == "" {
		return nil, fmt.Errorf("azure blob connector requires 'container_name'")
	}

	accountName, _ := config["account_name"].(string)
	accountKey, _ := config["account_key"].(string)
	connStr, _ := config["connection_string"].(string)
	prefix, _ := config["prefix"].(string)

	// If connection string provided, parse account name & key
	if connStr != "" {
		parts := strings.Split(connStr, ";")
		for _, p := range parts {
			if strings.HasPrefix(p, "AccountName=") {
				accountName = strings.TrimPrefix(p, "AccountName=")
			} else if strings.HasPrefix(p, "AccountKey=") {
				accountKey = strings.TrimPrefix(p, "AccountKey=")
			}
		}
	}

	return &AzureBlobConnector{
		name:             name,
		accountName:      accountName,
		accountKey:       accountKey,
		containerName:    containerName,
		prefix:           prefix,
		connectionString: connStr,
		client:           &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *AzureBlobConnector) Type() string { return "azure_blob" }
func (a *AzureBlobConnector) Name() string { return a.name }

func (a *AzureBlobConnector) ValidateConfig(ctx context.Context) error {
	if a.containerName == "" {
		return fmt.Errorf("azure container name is required")
	}
	return nil
}

type azureBlobEnumerationResults struct {
	XMLName xml.Name `xml:"EnumerationResults"`
	Blobs   struct {
		Blob []struct {
			Name       string `xml:"Name"`
			Properties struct {
				LastModified  time.Time `xml:"Last-Modified"`
				Etag          string    `xml:"Etag"`
				ContentLength int64     `xml:"Content-Length"`
				ContentType   string    `xml:"Content-Type"`
			} `xml:"Properties"`
		} `xml:"Blob"`
	} `xml:"Blobs"`
}

func (a *AzureBlobConnector) ListObjects(ctx context.Context, prefix string) ([]*StorageObject, error) {
	effectivePrefix := a.prefix
	if prefix != "" {
		effectivePrefix = path.Join(effectivePrefix, prefix)
	}

	baseURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s?restype=container&comp=list", a.accountName, a.containerName)
	if effectivePrefix != "" {
		baseURL += fmt.Sprintf("&prefix=%s", url.QueryEscape(effectivePrefix))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("x-ms-version", "2021-08-06")
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list azure blobs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("azure list blobs failed status %d: %s", resp.StatusCode, string(body))
	}

	var results azureBlobEnumerationResults
	if err := xml.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("failed to decode azure blob xml: %w", err)
	}

	var objects []*StorageObject
	for _, b := range results.Blobs.Blob {
		objects = append(objects, &StorageObject{
			Key:          b.Name,
			ETag:         strings.Trim(b.Properties.Etag, "\""),
			Size:         b.Properties.ContentLength,
			LastModified: b.Properties.LastModified,
			ContentType:  b.Properties.ContentType,
			Metadata: map[string]interface{}{
				"source":    "azure_blob",
				"container": a.containerName,
				"account":   a.accountName,
			},
		})
	}

	return objects, nil
}

func (a *AzureBlobConnector) FetchObject(ctx context.Context, key string) ([]byte, *StorageObject, error) {
	blobURL := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", a.accountName, a.containerName, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, blobURL, nil)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("x-ms-version", "2021-08-06")
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to fetch azure blob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("azure get blob returned status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read azure blob body: %w", err)
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
			"source":    "azure_blob",
			"container": a.containerName,
		},
	}

	return data, obj, nil
}
