package diff

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/omnirag/go-engine/internal/database"
)

type RawChunkInput struct {
	TextContent string                 `json:"text_content"`
	ChunkIndex  int                    `json:"chunk_index"`
	TokenCount  int                    `json:"token_count"`
	Metadata    map[string]interface{} `json:"metadata"`
}

type DiffResult struct {
	DocumentID         string            `json:"document_id"`
	VersionID          string            `json:"version_id"`
	VersionNumber      int               `json:"version_number"`
	FileHash           string            `json:"file_hash"`
	TotalChunks        int               `json:"total_chunks"`
	ReusedChunksCount  int               `json:"reused_chunks_count"`
	NewChunksCount     int               `json:"new_chunks_count"`
	NewChunksToEmbed   []*database.Chunk `json:"new_chunks_to_embed"`
	AllVersionChunkIDs []string          `json:"all_version_chunk_ids"`
	IsDocumentChanged  bool              `json:"is_document_changed"`
}

type Differ struct {
	store database.Store
}

func NewDiffer(store database.Store) *Differ {
	return &Differ{store: store}
}

// ProcessDocumentDiff performs smart chunk-level diffing against the existing document chunks
func (d *Differ) ProcessDocumentDiff(
	ctx context.Context,
	doc *database.Document,
	fileBytes []byte,
	rawChunks []RawChunkInput,
) (*DiffResult, error) {
	fileHash := ComputeFileHash(fileBytes)

	// Check if document already has a version with the exact same file hash
	latestVersion, err := d.store.GetLatestDocumentVersion(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest version: %w", err)
	}

	if latestVersion != nil && latestVersion.FileHash == fileHash {
		// Document is completely unchanged! 0 embedding cost and 0 version creation
		return &DiffResult{
			DocumentID:        doc.ID,
			VersionID:         latestVersion.ID,
			VersionNumber:     latestVersion.VersionNumber,
			FileHash:          fileHash,
			TotalChunks:       latestVersion.TotalChunks,
			ReusedChunksCount: latestVersion.TotalChunks,
			NewChunksCount:    0,
			NewChunksToEmbed:  nil,
			IsDocumentChanged: false,
		}, nil
	}

	newVersionNumber := 1
	if latestVersion != nil {
		newVersionNumber = latestVersion.VersionNumber + 1
	}

	// 1. Fetch all known chunk hashes for this document across previous versions
	existingChunksByHash, err := d.store.GetExistingChunksForDoc(ctx, doc.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch existing chunks: %w", err)
	}

	var allVersionChunkIDs []string
	var newChunksToEmbed []*database.Chunk
	reusedCount := 0
	newCount := 0

	// 2. Iterate through each chunk of the new document
	for i, rc := range rawChunks {
		chunkHash := ComputeChunkHash(rc.TextContent)

		if existingChunk, found := existingChunksByHash[chunkHash]; found {
			// ZERO COST RE-USE: Chunk content is identical to an existing chunk
			allVersionChunkIDs = append(allVersionChunkIDs, existingChunk.ID)
			reusedCount++
		} else {
			// NEW OR MODIFIED CHUNK: Needs embedding
			chunkID := uuid.New().String()
			newChunk := &database.Chunk{
				ID:          chunkID,
				DocumentID:  doc.ID,
				ChunkHash:   chunkHash,
				ChunkIndex:  i,
				TextContent: rc.TextContent,
				TokenCount:  rc.TokenCount,
				Metadata:    rc.Metadata,
				IsEmbedded:  false,
				CreatedAt:   time.Now().UTC(),
			}

			if err := d.store.UpsertChunk(ctx, newChunk); err != nil {
				return nil, fmt.Errorf("failed to store new chunk: %w", err)
			}

			// Add to in-memory map to avoid duplicate chunk records in same document
			existingChunksByHash[chunkHash] = newChunk

			allVersionChunkIDs = append(allVersionChunkIDs, chunkID)
			newChunksToEmbed = append(newChunksToEmbed, newChunk)
			newCount++
		}
	}

	// 3. Create new DocumentVersion
	versionID := uuid.New().String()
	newVersion := &database.DocumentVersion{
		ID:            versionID,
		DocumentID:    doc.ID,
		VersionNumber: newVersionNumber,
		FileHash:      fileHash,
		TotalChunks:   len(rawChunks),
		CreatedAt:     time.Now().UTC(),
	}

	if err := d.store.CreateDocumentVersion(ctx, newVersion); err != nil {
		return nil, fmt.Errorf("failed to create document version: %w", err)
	}

	// 4. Link chunks to new version in version_chunks table
	if err := d.store.LinkVersionChunks(ctx, versionID, allVersionChunkIDs); err != nil {
		return nil, fmt.Errorf("failed to link version chunks: %w", err)
	}

	// 5. Update document current_version_id
	doc.CurrentVersionID = versionID
	doc.Status = "synced"
	if err := d.store.UpdateDocument(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to update document: %w", err)
	}

	return &DiffResult{
		DocumentID:         doc.ID,
		VersionID:          versionID,
		VersionNumber:      newVersionNumber,
		FileHash:           fileHash,
		TotalChunks:        len(rawChunks),
		ReusedChunksCount:  reusedCount,
		NewChunksCount:     newCount,
		NewChunksToEmbed:   newChunksToEmbed,
		AllVersionChunkIDs: allVersionChunkIDs,
		IsDocumentChanged:  true,
	}, nil
}
