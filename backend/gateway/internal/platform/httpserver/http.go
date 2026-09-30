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
		WriteTimeout: 35 * time.Second, // must exceed max upstream call time (intake ~10s + margin)
		IdleTimeout:  60 * time.Second,
	}
}
