package diff

import (
	"context"
	"testing"

	"github.com/omnirag/go-engine/internal/database"
)

func TestChunkDiffingZeroCostReuse(t *testing.T) {
	store := database.NewMemoryStore()
	differ := NewDiffer(store)
	ctx := context.Background()

	doc := &database.Document{
		ID:          "doc_test_1",
		WorkspaceID: "ws_default",
		ConnectorID: "conn_1",
		ExternalID:  "s3://bucket/handbook.txt",
		FileName:    "handbook.txt",
		FileType:    "text/plain",
	}
	_ = store.CreateDocument(ctx, doc)

	// Version 1: 3 Chunks
	v1Chunks := []RawChunkInput{
		{TextContent: "Section 1: Mission Statement. We build great software.", ChunkIndex: 0, TokenCount: 8},
		{TextContent: "Section 2: Values. Speed, precision, and cost savings.", ChunkIndex: 1, TokenCount: 8},
		{TextContent: "Section 3: PTO Policy. 20 days off annually.", ChunkIndex: 2, TokenCount: 8},
	}
	fileBytesV1 := []byte("handbook content v1")

	resV1, err := differ.ProcessDocumentDiff(ctx, doc, fileBytesV1, v1Chunks)
	if err != nil {
		t.Fatalf("Failed to process v1 diff: %v", err)
	}

	if resV1.TotalChunks != 3 || resV1.NewChunksCount != 3 || resV1.ReusedChunksCount != 0 {
		t.Fatalf("Expected v1 to have 3 new chunks, got total=%d, new=%d, reused=%d",
			resV1.TotalChunks, resV1.NewChunksCount, resV1.ReusedChunksCount)
	}

	// Version 2: Only change Section 3 (20 days -> 25 days). Section 1 & Section 2 remain untouched!
	v2Chunks := []RawChunkInput{
		{TextContent: "Section 1: Mission Statement. We build great software.", ChunkIndex: 0, TokenCount: 8},
		{TextContent: "Section 2: Values. Speed, precision, and cost savings.", ChunkIndex: 1, TokenCount: 8},
		{TextContent: "Section 3: PTO Policy. 25 days off annually.", ChunkIndex: 2, TokenCount: 8},
	}
	fileBytesV2 := []byte("handbook content v2")

	resV2, err := differ.ProcessDocumentDiff(ctx, doc, fileBytesV2, v2Chunks)
	if err != nil {
		t.Fatalf("Failed to process v2 diff: %v", err)
	}

	if resV2.VersionNumber != 2 {
		t.Fatalf("Expected version number 2, got %d", resV2.VersionNumber)
	}

	if resV2.ReusedChunksCount != 2 {
		t.Fatalf("Expected 2 reused chunks with $0 cost, got %d", resV2.ReusedChunksCount)
	}

	if resV2.NewChunksCount != 1 {
		t.Fatalf("Expected only 1 new chunk to be embedded, got %d", resV2.NewChunksCount)
	}

	t.Logf("PASS: Go Chunk Differ successfully reused %d/%d chunks (Zero-cost reuse: %.1f%%)",
		resV2.ReusedChunksCount, resV2.TotalChunks, float64(resV2.ReusedChunksCount)/float64(resV2.TotalChunks)*100.0)
}
