package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Storage interface {
	SaveReportFile(auditID uuid.UUID, version int, filename string, data []byte) (string, error)
	ReadReportFile(relPath string) ([]byte, error)
}

type localStorage struct {
	baseDir string
	logger  *zap.Logger
}

func NewLocalStorage(baseDir string, logger *zap.Logger) Storage {
	return &localStorage{
		baseDir: baseDir,
		logger:  logger,
	}
}

// SaveReportFile writes report bytes to {baseDir}/{auditID}/v{version}/{filename}.
func (s *localStorage) SaveReportFile(auditID uuid.UUID, version int, filename string, data []byte) (string, error) {
	relDir := filepath.Join(auditID.String(), fmt.Sprintf("v%d", version))
	fullDir := filepath.Join(s.baseDir, relDir)

	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", fullDir, err)
	}

	relPath := filepath.Join(relDir, filename)
	fullPath := filepath.Join(s.baseDir, relPath)

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write report file %s: %w", fullPath, err)
	}

	s.logger.Info("report artifact stored",
		zap.String("path", fullPath),
		zap.Int("bytes", len(data)),
	)

	return relPath, nil
}

// ReadReportFile reads report artifact bytes from storage.
func (s *localStorage) ReadReportFile(relPath string) ([]byte, error) {
	fullPath := filepath.Join(s.baseDir, relPath)
	return os.ReadFile(fullPath)
}
