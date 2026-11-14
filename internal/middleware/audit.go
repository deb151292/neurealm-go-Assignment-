package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger logs HTTP requests with timing, status codes, and IP
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		log.Printf("[HTTP] %s %s | status=%d | duration=%v | client_ip=%s",
			c.Request.Method,
			path,
			c.Writer.Status(),
			time.Since(start),
			c.ClientIP(),
		)
	}
}
