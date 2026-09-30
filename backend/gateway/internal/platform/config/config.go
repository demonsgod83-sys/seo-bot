package config

import (
	"net/url"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port            string
	GinMode         string
	Logger          *zap.Logger
	IntakeBaseURL   string
	OrchestratorURL string
	// APIKeys maps raw key string → tenant UUID.
	// Parsed at startup from the API_KEYS env var:
	//   API_KEYS=key_live_abc:uuid1,key_live_xyz:uuid2
	APIKeys map[string]uuid.UUID
	// RateLimitRPS is the per-API-key token-bucket replenishment rate (req/s).
	// RateLimitBurst is the maximum burst size.
	RateLimitRPS   float64
	RateLimitBurst int
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
		Port:            getEnvOrDefault("PORT", "8080"),
		GinMode:         getEnvOrDefault("GIN_MODE", "debug"),
		Logger:          logger,
		IntakeBaseURL:   getEnvOrDefault("INTAKE_BASE_URL", "http://localhost:7007"),
		OrchestratorURL: getEnvOrDefault("ORCHESTRATOR_BASE_URL", "http://localhost:8090"),
		APIKeys:         parseAPIKeys(os.Getenv("API_KEYS"), logger),
		RateLimitRPS:    10,
		RateLimitBurst:  20,
	}
}

//==========================================//
//             PRIVATE FUNCTIONS            //
//==========================================//

// parseAPIKeys parses the API_KEYS env var into a lookup map.
// Format: key1:tenantUUID1,key2:tenantUUID2
// Invalid entries are logged and skipped so one bad key doesn't kill startup.
func parseAPIKeys(raw string, logger *zap.Logger) map[string]uuid.UUID {
	keys := make(map[string]uuid.UUID)
	if raw == "" {
		logger.Warn("API_KEYS is empty — all requests will be rejected with 401")
		return keys
	}

	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			logger.Warn("invalid API_KEYS entry (expected key:uuid)", zap.String("entry", pair))
			continue
		}
		key := strings.TrimSpace(parts[0])
		tenantID, err := uuid.Parse(strings.TrimSpace(parts[1]))
		if err != nil {
			logger.Warn("invalid tenant UUID in API_KEYS", zap.String("entry", pair), zap.Error(err))
			continue
		}
		keys[key] = tenantID
	}

	logger.Info("API keys loaded", zap.Int("count", len(keys)))
	return keys
}

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
