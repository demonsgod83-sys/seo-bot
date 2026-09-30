package storage

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

type StorageEngine interface {
	GetSnapshot(ctx context.Context, storagePath string) ([]byte, error)
}

type localStorage struct {
	baseDir string
	logger  *zap.Logger
}

func NewLocalStorage(baseDir string, logger *zap.Logger) StorageEngine {
	return &localStorage{
		baseDir: baseDir,
		logger:  logger,
	}
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
