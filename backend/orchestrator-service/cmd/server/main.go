package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/demonsgod83-sys/seo-bot/orchestrator-service/internal/orchestrator"
	"github.com/demonsgod83-sys/seo-bot/orchestrator-service/internal/platform/config"
	"github.com/demonsgod83-sys/seo-bot/orchestrator-service/internal/platform/database"
	"github.com/demonsgod83-sys/seo-bot/orchestrator-service/internal/platform/httpserver"
	"github.com/demonsgod83-sys/seo-bot/orchestrator-service/internal/platform/natsplatform"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	/* INITIALIZE ENV AND CONTEXTS */
	cfg := config.LoadConfig()
	rootCtx, rootCancel := context.WithCancel(context.Background())

	/* INITIALIZE SERVICES */

	// DATABASE
	db := database.NewSQLDatabase(cfg.Logger)

	if err := db.Connect(rootCtx, cfg.DatabaseDSN); err != nil {
		log.Fatal("Database connection failed!", zap.Error(err))
	}

	// Create the audit_transitions table (Orchestrator owns this table).
	// The audits table is owned by Intake; Orchestrator only adds columns to it.
	if err := db.RegisterModels(rootCtx,
		(*orchestrator.AuditTransition)(nil),
		(*orchestrator.AuditAnalyzerRun)(nil),
	); err != nil {
		cfg.Logger.Warn("Error creating tables!", zap.Error(err))
	}

	// Run migrations: adds step_started_at and step_attempts columns to audits.
	if err := db.Migrate(rootCtx); err != nil {
		cfg.Logger.Warn("Error running migrations!", zap.Error(err))
	}

	// NATS
	natsClient, err := natsplatform.Connect(cfg.NatsURL, cfg.Logger)
	if err != nil {
		cfg.Logger.Fatal("NATS connection failed!", zap.Error(err))
	}

	// DEPENDENCY INJECTIONS
	orchestratorRepository := orchestrator.NewRepository(db.GetDBClient(), cfg.Logger)
	orchestratorService := orchestrator.NewService(orchestratorRepository, natsClient.JS, cfg.Logger)
	orchestratorHandler := orchestrator.NewHandler(orchestratorService)

	// NATS CONSUMER
	consumer, err := orchestrator.NewConsumer(natsClient.JS, orchestratorService, cfg.Logger)
	if err != nil {
		cfg.Logger.Fatal("Failed to start NATS consumer!", zap.Error(err))
	}

	// GIN ENGINE
	gin.SetMode(cfg.GinMode)
	ginEngine := httpserver.NewGinEngine(cfg.Logger)
	orchestratorRoutes := orchestrator.NewRoutes(ginEngine, orchestratorHandler)

	/* INITIALIZE ROUTES */
	orchestratorRoutes.SetupPublicRoutes()

	/* START SERVERS */
	httpServer := httpserver.NewHTTPServer(cfg.Port, ginEngine)

	go func() {
		cfg.Logger.Info("HTTP server listening", zap.String("port", cfg.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			cfg.Logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	/* SHUTDOWN MECHANISM */
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	cfg.Logger.Info("Shutdown signal received")

	// Drain NATS consumer first — let in-flight messages finish
	consumer.Drain()

	// Stop HTTP server
	shutDownHTTP(httpServer, cfg.Logger)

	// Cancel background context
	rootCancel()

	// Drain NATS connection — flush any pending published messages
	natsClient.Drain()

	// Flush logger
	cfg.Logger.Sync()
}

func shutDownHTTP(s *http.Server, logger *zap.Logger) {
	if s != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := s.Shutdown(ctx)
		if err != nil {
			logger.Error("HTTP shutdown error", zap.Error(err))
			return
		}
	}
	logger.Info("Server gracefully shut down")
}
