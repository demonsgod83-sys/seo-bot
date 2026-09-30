package httpserver

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewGinEngine(logger *zap.Logger) *gin.Engine {
	ginEngine := gin.New()

	// ALL MIDDLEWARES
	ginEngine.Use(RequestID()) // Gives custom ID to requests
	ginEngine.Use(ZapLogger(logger))
	ginEngine.Use(Recovery(logger)) // Panic recovery with structured logging
	ginEngine.Use(CORSMiddleware())

	return ginEngine
}
