package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/demonsgod83-sys/seo-bot/keyword-extractor/internal/keywords"
	"github.com/demonsgod83-sys/seo-bot/keyword-extractor/internal/platform/config"
	"github.com/demonsgod83-sys/seo-bot/keyword-extractor/internal/platform/database"
	"github.com/demonsgod83-sys/seo-bot/keyword-extractor/internal/platform/httpserver"
	"github.com/demonsgod83-sys/seo-bot/keyword-extractor/internal/platform/natsplatform"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	/* INITIALIZE ENV AND CONTEXTS */
	cfg := config.LoadConfig()
	rootCtx, rootCancel := context.WithCancel(context.Background())

	/* INITIALIZE PLATFORMS */

	// 1. DATABASE
	db := database.NewSQLDatabase(cfg.Logger)
	if err := db.Connect(rootCtx, cfg.DatabaseDSN); err != nil {
		log.Fatal("Database connection failed!", zap.Error(err))
	}

	// Register models
	if err := db.RegisterModels(rootCtx,
		(*keywords.PageKeyword)(nil),
		(*keywords.SiteKeywordSummary)(nil),
		(*keywords.Finding)(nil),
	); err != nil {
		cfg.Logger.Warn("Error creating tables!", zap.Error(err))
	}

	// 2. NATS JETSTREAM
	natsClient, err := natsplatform.Connect(cfg.NatsURL, cfg.Logger)
	if err != nil {
		cfg.Logger.Fatal("NATS connection failed!", zap.Error(err))
	}

	// DEPENDENCY INJECTIONS
	keywordRepository := keywords.NewRepository(db.GetDBClient(), cfg.Logger)
	keywordService := keywords.NewService(keywordRepository, natsClient.JS, cfg.Logger)
	keywordHandler := keywords.NewHandler(keywordService)

	// NATS CONSUMER
	consumer, err := keywords.NewConsumer(natsClient.JS, keywordService, cfg.Logger)
	if err != nil {
		cfg.Logger.Fatal("Failed to start keyword extractor NATS consumer!", zap.Error(err))
	}

	// GIN ENGINE & ROUTES
	gin.SetMode(cfg.GinMode)
	ginEngine := httpserver.NewGinEngine(cfg.Logger)
	keywordRoutes := keywords.NewRoutes(ginEngine, keywordHandler)
	keywordRoutes.SetupPublicRoutes()

	/* START HTTP SERVER */
	httpServer := httpserver.NewHTTPServer(cfg.Port, ginEngine)
	go func() {
		cfg.Logger.Info("Keyword Extractor HTTP server listening", zap.String("port", cfg.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			cfg.Logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	/* SHUTDOWN MECHANISM */
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	cfg.Logger.Info("Shutdown signal received")

	consumer.Drain()
	shutDownHTTP(httpServer, cfg.Logger)
	rootCancel()
	natsClient.Drain()
	_ = cfg.Logger.Sync()
}

func shutDownHTTP(s *http.Server, logger *zap.Logger) {
	if s != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.Shutdown(ctx); err != nil {
			logger.Error("HTTP shutdown error", zap.Error(err))
			return
		}
	}
	logger.Info("Server gracefully shut down")
}
