package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
	"github.com/gin-gonic/gin"
)

func Recoverer() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log.Printf("[PANIC RECOVERED] %v\nStack trace:\n%s", rvr, debug.Stack())
				response.Error(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an unexpected internal error occurred", nil)
				c.Abort()
			}
		}()

		c.Next()
	}
}
