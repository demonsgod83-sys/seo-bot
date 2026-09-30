package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port              string
	GinMode           string
	DatabaseDSN       string
	NatsURL           string
	CriticalDeduction float64
	WarningDeduction  float64
	InfoDeduction     float64
	WeightTechnical   float64
	WeightOnPage      float64
	WeightContent     float64
	Logger            *zap.Logger
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	logger, _ := zap.NewProduction()

	port := getEnv("PORT", "8100")
	ginMode := getEnv("GIN_MODE", "release")
	databaseDSN := getEnv("DATABASE_DSN", "postgres://seo_user:seo_pass@localhost:5432/seo_bot?sslmode=disable")
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")

	criticalDeduction := getEnvFloat("CRITICAL_DEDUCTION", 15.0)
	warningDeduction := getEnvFloat("WARNING_DEDUCTION", 5.0)
	infoDeduction := getEnvFloat("INFO_DEDUCTION", 1.0)

	weightTechnical := getEnvFloat("WEIGHT_TECHNICAL", 0.40)
	weightOnPage := getEnvFloat("WEIGHT_ONPAGE", 0.35)
	weightContent := getEnvFloat("WEIGHT_CONTENT", 0.25)

	return &Config{
		Port:              port,
		GinMode:           ginMode,
		DatabaseDSN:       databaseDSN,
		NatsURL:           natsURL,
		CriticalDeduction: criticalDeduction,
		WarningDeduction:  warningDeduction,
		InfoDeduction:     infoDeduction,
		WeightTechnical:   weightTechnical,
		WeightOnPage:      weightOnPage,
		WeightContent:     weightContent,
		Logger:            logger,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return defaultVal
}
