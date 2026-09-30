package config

import (
	"net/url"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

type Config struct {
	Port        string
	GinMode     string
	DatabaseDSN string
	NatsURL     string
	Logger      *zap.Logger
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
		Port:        getEnvOrDefault("PORT", "8098"),
		GinMode:     getEnvOrDefault("GIN_MODE", "debug"),
		NatsURL:     getEnvOrDefault("NATS_URL", "nats://localhost:4222"),
		Logger:      logger,
		DatabaseDSN: makeDatabaseDSN(),
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
