package middleware

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS は許可オリジンに対して CORS ヘッダを付与する。
// preflight(OPTIONS) は 204 で即応答する。
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allowAll := slices.Contains(allowedOrigins, "*")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowAll || slices.Contains(allowedOrigins, origin)) {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Vary", "Origin")
			c.Writer.Header().Set("Access-Control-Allow-Methods", strings.Join([]string{
				http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions,
			}, ", "))
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+RequestIDHeader)
			c.Writer.Header().Set("Access-Control-Expose-Headers", RequestIDHeader)
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
