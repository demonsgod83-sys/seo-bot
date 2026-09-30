package httpserver

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func NewHTTPServer(port string, ginEngine *gin.Engine) *http.Server {
	return &http.Server{
		Addr:    ":" + port,
		Handler: ginEngine,
		// ReadTimeout covers reading the request body (fast for this service).
		ReadTimeout: 10 * time.Second,
		// WriteTimeout must exceed the slowest handler. The intake endpoint
		// makes an outbound reachability check (max 5s HEAD + 5s GET = 10s).
		// 30s gives clear headroom without being dangerously open-ended.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
