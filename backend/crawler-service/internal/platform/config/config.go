package config

import (
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port           string
	GinMode        string
	DatabaseDSN    string
	NatsURL        string
	StorageDir     string
	UserAgent      string
	MaxPagesPerJob int
	MaxConcurrency int
	DefaultRPS     float64
	PageTimeoutSec int
	Logger         *zap.Logger
}

func LoadConfig() *Config {
	env := os.Getenv("ENV")
	if env != "production" {
		_ = godotenv.Load()
		env = os.Getenv("ENV")
	}

	var logger *zap.Logger
	var err error
	if env == "production" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		panic(err)
	}

	return &Config{
		Port:           getEnvOrDefault("PORT", "8095"),
		GinMode:        getEnvOrDefault("GIN_MODE", "debug"),
		NatsURL:        getEnvOrDefault("NATS_URL", "nats://localhost:4222"),
		StorageDir:     getEnvOrDefault("STORAGE_DIR", "./data/snapshots"),
		UserAgent:      getEnvOrDefault("USER_AGENT", "SEO-Bot/1.0 (+https://seobot.dev/bot-info; bot@seobot.dev)"),
		MaxPagesPerJob: getEnvIntOrDefault("MAX_PAGES_PER_JOB", 50),
		MaxConcurrency: getEnvIntOrDefault("MAX_CONCURRENCY", 3),
		DefaultRPS:     getEnvFloatOrDefault("DEFAULT_RPS", 2.0),
		PageTimeoutSec: getEnvIntOrDefault("PAGE_TIMEOUT_SEC", 10),
		Logger:         logger,
		DatabaseDSN:    makeDatabaseDSN(),
	}
}

//==========================================//
//             PRIVATE FUNCTIONS            //
//==========================================//

func makeDatabaseDSN() string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(os.Getenv("POSTGRES_USER"), os.Getenv("POSTGRES_PASSWORD")),
		Host:     os.Getenv("POSTGRES_HOST") + ":" + os.Getenv("POSTGRES_PORT"),
		Path:     os.Getenv("POSTGRES_DB"),
		RawQuery: "sslmode=disable",
	}
	return u.String()
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getEnvFloatOrDefault(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
