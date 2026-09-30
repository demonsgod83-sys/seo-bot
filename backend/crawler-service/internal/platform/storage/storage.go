package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StorageEngine defines the contract for storing and retrieving raw page snapshots.
type StorageEngine interface {
	SaveSnapshot(ctx context.Context, auditID uuid.UUID, pageURL string, html []byte) (storagePath string, err error)
	GetSnapshot(ctx context.Context, storagePath string) ([]byte, error)
}

type localStorage struct {
	baseDir string
	logger  *zap.Logger
}

// NewLocalStorage initializes a local disk snapshot storage engine.
// Snapshots are stored compressed (.html.gz) under baseDir/auditID/hash.html.gz
func NewLocalStorage(baseDir string, logger *zap.Logger) (StorageEngine, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage base dir (%s): %w", baseDir, err)
	}
	return &localStorage{
		baseDir: baseDir,
		logger:  logger,
	}, nil
}

func (s *localStorage) SaveSnapshot(ctx context.Context, auditID uuid.UUID, pageURL string, html []byte) (string, error) {
	auditDir := filepath.Join(s.baseDir, auditID.String())
	if err := os.MkdirAll(auditDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create audit snapshot dir: %w", err)
	}

	// Compute stable hash of page URL for filename
	hasher := md5.New()
	hasher.Write([]byte(pageURL))
	filename := hex.EncodeToString(hasher.Sum(nil)) + ".html.gz"
	fullPath := filepath.Join(auditDir, filename)

	// Gzip compress the HTML snapshot
	var buf bytes.Buffer
	gzWriter := gzip.NewWriter(&buf)
	if _, err := gzWriter.Write(html); err != nil {
		return "", fmt.Errorf("gzip compression failed: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		return "", fmt.Errorf("gzip writer close failed: %w", err)
	}

	if err := os.WriteFile(fullPath, buf.Bytes(), 0644); err != nil {
		return "", fmt.Errorf("failed to write snapshot file (%s): %w", fullPath, err)
	}

	relPath := filepath.Join(auditID.String(), filename)
	return relPath, nil
}

func (s *localStorage) GetSnapshot(ctx context.Context, storagePath string) ([]byte, error) {
	fullPath := filepath.Join(s.baseDir, storagePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read snapshot file (%s): %w", fullPath, err)
	}

	gzReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader init failed: %w", err)
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, fmt.Errorf("gzip decompression failed: %w", err)
	}

	return decompressed, nil
}
