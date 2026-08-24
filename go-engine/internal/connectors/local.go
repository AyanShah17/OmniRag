package connectors

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/omnirag/go-engine/internal/diff"
)

type LocalConnector struct {
	name     string
	basePath string
}

func NewLocalConnector(name string, config map[string]interface{}) (*LocalConnector, error) {
	basePath, _ := config["base_path"].(string)
	if basePath == "" {
		basePath, _ = config["path"].(string)
	}
	if basePath == "" {
		basePath = "./storage"
	}

	// Ensure directory exists
	_ = os.MkdirAll(basePath, 0755)

	return &LocalConnector{
		name:     name,
		basePath: basePath,
	}, nil
}

func (l *LocalConnector) Type() string { return "local" }
func (l *LocalConnector) Name() string { return l.name }

func (l *LocalConnector) ValidateConfig(ctx context.Context) error {
	info, err := os.Stat(l.basePath)
	if err != nil {
		return fmt.Errorf("local base path inaccessible: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("local base path is not a directory: %s", l.basePath)
	}
	return nil
}

func (l *LocalConnector) ListObjects(ctx context.Context, prefix string) ([]*StorageObject, error) {
	searchDir := l.basePath
	if prefix != "" {
		searchDir = filepath.Join(l.basePath, prefix)
	}

	var objects []*StorageObject

	err := filepath.WalkDir(searchDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(l.basePath, p)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Calculate file etag/hash from content
		data, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		fileHash := diff.ComputeFileHash(data)

		objects = append(objects, &StorageObject{
			Key:          relPath,
			ETag:         fileHash,
			Size:         info.Size(),
			LastModified: info.ModTime(),
			ContentType:  detectContentType(relPath),
			Metadata: map[string]interface{}{
				"source":    "local",
				"full_path": p,
			},
		})
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to scan local directory: %w", err)
	}

	return objects, nil
}

func (l *LocalConnector) FetchObject(ctx context.Context, key string) ([]byte, *StorageObject, error) {
	fullPath := filepath.Join(l.basePath, filepath.FromSlash(key))
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read local file %s: %w", fullPath, err)
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat local file %s: %w", fullPath, err)
	}

	etag := diff.ComputeFileHash(data)

	obj := &StorageObject{
		Key:          strings.ReplaceAll(key, "\\", "/"),
		ETag:         etag,
		Size:         int64(len(data)),
		LastModified: info.ModTime(),
		ContentType:  detectContentType(key),
		Metadata: map[string]interface{}{
			"source":    "local",
			"full_path": fullPath,
		},
	}

	return data, obj, nil
}
