package gatewaymiddleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

//==========================================//
//           AUTHENTICATION MIDDLEWARE      //
//==========================================//

// Auth returns a Gin middleware that validates the API key on every request.
//
// Key is read from the "X-API-Key" header.
// On success it sets two context values used by handlers downstream:
//   - "tenant_id"  (uuid.UUID) — injected into every call to Intake/Orchestrator
//   - "api_key"    (string)    — available for logging and rate-limit keying
//
// Rule: the tenant_id propagated here is the ONLY source of tenant information
// in the system. Handlers must never trust a client-supplied tenant_id.
func Auth(apiKeys map[string]uuid.UUID, logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			// Also accept Bearer token format for tooling compatibility
			key = extractBearer(c.GetHeader("Authorization"))
		}

		if key == "" {
			fail(c, http.StatusUnauthorized, "MISSING_API_KEY", "X-API-Key header is required")
			return
		}

		tenantID, ok := apiKeys[key]
		if !ok {
			logger.Warn("rejected request with invalid API key",
				zap.String("key_prefix", safePrefix(key)),
				zap.String("path", c.Request.URL.Path),
			)
			fail(c, http.StatusUnauthorized, "INVALID_API_KEY", "the provided API key is not valid")
			return
		}

		// Inject tenant identity for downstream handlers
		c.Set("tenant_id", tenantID)
		c.Set("api_key", key)
		c.Next()
	}
}

//==========================================//
//             HELPER FUNCTIONS             //
//==========================================//

func extractBearer(authHeader string) string {
	const prefix = "Bearer "
	if len(authHeader) > len(prefix) && authHeader[:len(prefix)] == prefix {
		return authHeader[len(prefix):]
	}
	return ""
}

// safePrefix returns the first 8 chars of a key for logging — enough to
// identify a key in logs without exposing the full secret.
func safePrefix(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:8] + "****"
}

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
		"request_id": c.GetString("request_id"),
	})
}
