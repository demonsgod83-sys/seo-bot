package httpserver

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func NewGinEngine(logger *zap.Logger) *gin.Engine {
	r := gin.New()
	r.Use(GinZapLogger(logger))
	r.Use(gin.Recovery())
	return r
}
