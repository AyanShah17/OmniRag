package connectors

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type StorageObject struct {
	Key          string                 `json:"key"`           // Path or identifier (e.g. "reports/q3_earnings.pdf")
	ETag         string                 `json:"etag"`          // Hash or version identifier
	Size         int64                  `json:"size"`          // File size in bytes
	LastModified time.Time              `json:"last_modified"` // Last modification timestamp
	ContentType  string                 `json:"content_type"`  // MIME type (application/pdf, text/markdown, etc.)
	Metadata     map[string]interface{} `json:"metadata"`      // Custom tags / ACL metadata
}

type Connector interface {
	Type() string
	Name() string
	ValidateConfig(ctx context.Context) error
	ListObjects(ctx context.Context, prefix string) ([]*StorageObject, error)
	FetchObject(ctx context.Context, key string) ([]byte, *StorageObject, error)
}

type Factory func(name string, config map[string]interface{}) (Connector, error)

var (
	factoriesMu sync.RWMutex
	factories   = map[string]Factory{}
)

func RegisterFactory(connectorType string, factory Factory) {
	factoriesMu.Lock()
	defer factoriesMu.Unlock()
	factories[connectorType] = factory
}

func init() {
	for _, connectorType := range []string{"s3", "aws_s3"} {
		RegisterFactory(connectorType, func(name string, config map[string]interface{}) (Connector, error) {
			return NewS3Connector(name, config)
		})
	}
	for _, connectorType := range []string{"azure", "azure_blob"} {
		RegisterFactory(connectorType, func(name string, config map[string]interface{}) (Connector, error) {
			return NewAzureBlobConnector(name, config)
		})
	}
	for _, connectorType := range []string{"supabase", "supabase_storage"} {
		RegisterFactory(connectorType, func(name string, config map[string]interface{}) (Connector, error) {
			return NewSupabaseStorageConnector(name, config)
		})
	}
	for _, connectorType := range []string{"confluence", "confluence_cloud"} {
		RegisterFactory(connectorType, func(name string, config map[string]interface{}) (Connector, error) {
			return NewConfluenceConnector(name, config)
		})
	}
	for _, connectorType := range []string{"local", "filesystem"} {
		RegisterFactory(connectorType, func(name string, config map[string]interface{}) (Connector, error) {
			return NewLocalConnector(name, config)
		})
	}
}

func CreateConnector(connectorType string, name string, config map[string]interface{}) (Connector, error) {
	factoriesMu.RLock()
	factory, ok := factories[connectorType]
	factoriesMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unsupported connector type: %s", connectorType)
	}
	return factory(name, config)
}
