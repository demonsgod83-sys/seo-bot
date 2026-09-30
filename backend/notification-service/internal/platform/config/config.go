package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port               string
	GinMode            string
	DatabaseDSN        string
	NatsURL            string
	SMTPEnabled        bool
	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPass           string
	SMTPFrom           string
	DefaultNotifyEmail string
	Logger             *zap.Logger
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	logger, _ := zap.NewProduction()

	port := getEnv("PORT", "8110")
	ginMode := getEnv("GIN_MODE", "release")
	databaseDSN := getEnv("DATABASE_DSN", "postgres://seo_user:seo_pass@localhost:5432/seo_bot?sslmode=disable")
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")

	smtpEnabled := getEnvBool("SMTP_ENABLED", false)
	smtpHost := getEnv("SMTP_HOST", "smtp.mailtrap.io")
	smtpPort := getEnvInt("SMTP_PORT", 2525)
	smtpUser := getEnv("SMTP_USER", "")
	smtpPass := getEnv("SMTP_PASS", "")
	smtpFrom := getEnv("SMTP_FROM", "alerts@seobot.dev")
	defaultNotifyEmail := getEnv("DEFAULT_NOTIFY_EMAIL", "client@example.com")

	return &Config{
		Port:               port,
		GinMode:            ginMode,
		DatabaseDSN:        databaseDSN,
		NatsURL:            natsURL,
		SMTPEnabled:        smtpEnabled,
		SMTPHost:           smtpHost,
		SMTPPort:           smtpPort,
		SMTPUser:           smtpUser,
		SMTPPass:           smtpPass,
		SMTPFrom:           smtpFrom,
		DefaultNotifyEmail: defaultNotifyEmail,
		Logger:             logger,
	}
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		b, err := strconv.ParseBool(val)
		if err == nil {
			return b
		}
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		i, err := strconv.Atoi(val)
		if err == nil {
			return i
		}
	}
	return defaultVal
}
