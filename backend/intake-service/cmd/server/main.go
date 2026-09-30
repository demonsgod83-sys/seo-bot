package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/demonsgod83-sys/seo-bot/intake-service/internal/intake"
	"github.com/demonsgod83-sys/seo-bot/intake-service/internal/platform/config"
	"github.com/demonsgod83-sys/seo-bot/intake-service/internal/platform/database"
	"github.com/demonsgod83-sys/seo-bot/intake-service/internal/platform/httpserver"
	"github.com/demonsgod83-sys/seo-bot/intake-service/internal/platform/natsplatform"
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

	// Table creation
	if err := db.RegisterModels(rootCtx,
		(*intake.Audit)(nil),
	); err != nil {
		cfg.Logger.Warn("Error creating tables!", zap.Error(err))
	}

	// NATS — non-fatal: if NATS is down, intake still accepts submissions.
	// The publisher will be nil and the service will log a warning per submission.
	var publisher intake.EventPublisher
	natsClient, err := natsplatform.Connect(cfg.NatsURL, cfg.Logger)
	if err != nil {
		cfg.Logger.Warn("NATS unavailable — audit.validated events will NOT be published", zap.Error(err))
	} else {
		publisher = intake.NewNatsPublisher(natsClient.JS)
	}

	// DEPENDENCY INJECTIONS
	intakeRepository := intake.NewRepository(db.GetDBClient(), cfg.Logger)
	intakeService := intake.NewService(intakeRepository, publisher, cfg.Logger)
	intakeHandler := intake.NewHandler(intakeService)

	// GIN ENGINE
	gin.SetMode(cfg.GinMode)
	ginEngine := httpserver.NewGinEngine(cfg.Logger)
	intakeRoutes := intake.NewRoutes(ginEngine, intakeHandler)

	/* INITIALIZE ROUTES */
	intakeRoutes.SetupPublicRoutes()

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

	// Stop HTTP server
	shutDownHTTP(httpServer, cfg.Logger)

	// Cancel background context
	rootCancel()

	// Drain NATS — flush any pending published messages
	if natsClient != nil {
		natsClient.Drain()
	}

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
