package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func NewHTTPServer(port string, ginEngine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:         ":" + port,
		Handler:      ginEngine,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
