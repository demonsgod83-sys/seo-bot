package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/demonsgod83-sys/seo-bot/gateway/internal/clients/intake"
	"github.com/demonsgod83-sys/seo-bot/gateway/internal/clients/orchestrator"
	"github.com/demonsgod83-sys/seo-bot/gateway/internal/gateway"
	gatewaymiddleware "github.com/demonsgod83-sys/seo-bot/gateway/internal/gateway/middleware"
	"github.com/demonsgod83-sys/seo-bot/gateway/internal/platform/config"
	"github.com/demonsgod83-sys/seo-bot/gateway/internal/platform/httpserver"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	/* INITIALIZE ENV AND CONTEXTS */
	cfg := config.LoadConfig()
	_, rootCancel := context.WithCancel(context.Background())

	/* INITIALIZE SERVICES */

	// Internal service clients
	// To swap intake for gRPC: replace intakeclient.NewHTTPClient with a gRPC
	// implementation that satisfies intakeclient.Client — nothing else changes.
	intakeClient := intakeclient.NewHTTPClient(cfg.IntakeBaseURL, cfg.Logger)
	orchestratorClient := orchestratorclient.NewHTTPClient(cfg.OrchestratorURL, cfg.Logger)

	// Rate limiter — in-memory token bucket, one per API key
	rateLimiter := gatewaymiddleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	// DEPENDENCY INJECTIONS
	gatewayHandler := gateway.NewHandler(intakeClient, orchestratorClient, cfg.Logger)

	// GIN ENGINE
	gin.SetMode(cfg.GinMode)
	ginEngine := httpserver.NewGinEngine(cfg.Logger)
	gatewayRoutes := gateway.NewRoutes(ginEngine, gatewayHandler, cfg.APIKeys, rateLimiter)

	/* INITIALIZE ROUTES */
	gatewayRoutes.SetupPublicRoutes()

	/* START SERVERS */
	httpServer := httpserver.NewHTTPServer(cfg.Port, ginEngine)

	go func() {
		cfg.Logger.Info("Gateway HTTP server listening",
			zap.String("port", cfg.Port),
			zap.String("intake_url", cfg.IntakeBaseURL),
			zap.String("orchestrator_url", cfg.OrchestratorURL),
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			cfg.Logger.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	/* SHUTDOWN MECHANISM */
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	cfg.Logger.Info("Shutdown signal received")

	shutDownHTTP(httpServer, cfg.Logger)
	rootCancel()
	cfg.Logger.Sync()
}

func shutDownHTTP(s *http.Server, logger *zap.Logger) {
	if s == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		logger.Error("HTTP shutdown error", zap.Error(err))
		return
	}
	log.Println("Gateway gracefully shut down")
}
