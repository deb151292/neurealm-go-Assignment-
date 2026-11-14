package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"github.com/deb151292/neurealm-go-Assignment-/internal/response"
)

// Recoverer catches unhandled panics and returns a clean JSON 500 response
func Recoverer() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					log.Printf("[PANIC RECOVERED] %v\nStack trace:\n%s", rvr, debug.Stack())
					response.Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "an unexpected internal error occurred", nil)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
