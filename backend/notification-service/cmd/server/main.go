package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/demonsgod83-sys/seo-bot/notification-service/internal/notification"
	"github.com/demonsgod83-sys/seo-bot/notification-service/internal/platform/config"
	"github.com/demonsgod83-sys/seo-bot/notification-service/internal/platform/database"
	"github.com/demonsgod83-sys/seo-bot/notification-service/internal/platform/httpserver"
	"github.com/demonsgod83-sys/seo-bot/notification-service/internal/platform/natsplatform"
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

	// Register models for table creation
	if err := db.RegisterModels(rootCtx,
		(*notification.TenantNotificationConfig)(nil),
		(*notification.NotificationDelivery)(nil),
	); err != nil {
		cfg.Logger.Warn("Error creating tables!", zap.Error(err))
	}

	// 2. NATS JETSTREAM
	natsClient, err := natsplatform.Connect(cfg.NatsURL, cfg.Logger)
	if err != nil {
		cfg.Logger.Fatal("NATS connection failed!", zap.Error(err))
	}

	// 3. SENDERS & REPOSITORY
	emailSender := notification.NewEmailSender(
		cfg.SMTPEnabled,
		cfg.SMTPHost,
		cfg.SMTPPort,
		cfg.SMTPUser,
		cfg.SMTPPass,
		cfg.SMTPFrom,
		cfg.Logger,
	)
	webhookSender := notification.NewWebhookSender(cfg.Logger)
	notificationRepository := notification.NewRepository(db.GetDBClient(), cfg.Logger)

	// DEPENDENCY INJECTIONS
	notificationService := notification.NewService(
		notificationRepository,
		emailSender,
		webhookSender,
		natsClient.JS,
		cfg.DefaultNotifyEmail,
		cfg.Logger,
	)
	notificationHandler := notification.NewHandler(notificationService)

	// NATS CONSUMER
	consumer, err := notification.NewConsumer(natsClient.JS, notificationService, cfg.Logger)
	if err != nil {
		cfg.Logger.Fatal("Failed to start notification service NATS consumer!", zap.Error(err))
	}

	// GIN ENGINE & ROUTES
	gin.SetMode(cfg.GinMode)
	ginEngine := httpserver.NewGinEngine(cfg.Logger)
	notificationRoutes := notification.NewRoutes(ginEngine, notificationHandler)
	notificationRoutes.SetupPublicRoutes()

	/* START HTTP SERVER */
	httpServer := httpserver.NewHTTPServer(cfg.Port, ginEngine)
	go func() {
		cfg.Logger.Info("Notification Service HTTP server listening", zap.String("port", cfg.Port))
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
