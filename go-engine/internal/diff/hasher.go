package diff

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"unicode"
)

// NormalizeText cleans up extra whitespace and carriage returns so formatting differences don't cause unnecessary re-embedding.
func NormalizeText(text string) string {
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Collapse multiple spaces/tabs while preserving line breaks
	lines := strings.Split(text, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimFunc(line, unicode.IsSpace)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}
	return strings.Join(cleanedLines, "\n")
}

// ComputeChunkHash returns the SHA-256 hash of normalized chunk text
func ComputeChunkHash(text string) string {
	normalized := NormalizeText(text)
	hasher := sha256.New()
	hasher.Write([]byte(normalized))
	return hex.EncodeToString(hasher.Sum(nil))
}

// ComputeFileHash returns SHA-256 hash of entire file bytes
func ComputeFileHash(data []byte) string {
	hasher := sha256.New()
	hasher.Write(data)
	return hex.EncodeToString(hasher.Sum(nil))
}
