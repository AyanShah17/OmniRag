package connectors

import (
	"context"
	"fmt"
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

func CreateConnector(connectorType string, name string, config map[string]interface{}) (Connector, error) {
	switch connectorType {
	case "s3", "aws_s3":
		return NewS3Connector(name, config)
	case "azure", "azure_blob":
		return NewAzureBlobConnector(name, config)
	case "supabase", "supabase_storage":
		return NewSupabaseStorageConnector(name, config)
	case "confluence", "confluence_cloud":
		return NewConfluenceConnector(name, config)
	case "local", "filesystem":
		return NewLocalConnector(name, config)
	default:
		return nil, fmt.Errorf("unsupported connector type: %s", connectorType)
	}
}
