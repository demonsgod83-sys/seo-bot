package httpserver

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewGinEngine(logger *zap.Logger) *gin.Engine {
	ginEngine := gin.New()

	// ALL MIDDLEWARES
	ginEngine.Use(RequestID())
	ginEngine.Use(ZapLogger(logger))
	ginEngine.Use(Recovery(logger))
	ginEngine.Use(CORSMiddleware())

	return ginEngine
}
