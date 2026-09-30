package config

import (
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port          string
	GinMode       string
	DatabaseDSN   string
	NatsURL       string
	StorageDir    string
	PublicBaseURL string
	Logger        *zap.Logger
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	logger, _ := zap.NewProduction()

	port := getEnv("PORT", "8105")
	ginMode := getEnv("GIN_MODE", "release")
	databaseDSN := getEnv("DATABASE_DSN", "postgres://seo_user:seo_pass@localhost:5432/seo_bot?sslmode=disable")
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	storageDir := getEnv("STORAGE_DIR", "data/reports")
	publicBaseURL := getEnv("PUBLIC_BASE_URL", "http://localhost:8105")

	return &Config{
		Port:          port,
		GinMode:       ginMode,
		DatabaseDSN:   databaseDSN,
		NatsURL:       natsURL,
		StorageDir:    storageDir,
		PublicBaseURL: publicBaseURL,
		Logger:        logger,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}
