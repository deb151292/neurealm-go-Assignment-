package docs

import (
	_ "embed"
	"net/http"

	"github.com/go-chi/chi/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

//go:embed openapi.yaml
var OpenAPIYAML []byte

//go:embed swagger.json
var SwaggerJSON []byte

// RegisterSwaggerRoutes mounts the Swagger UI and OpenAPI documentation endpoints onto the router
func RegisterSwaggerRoutes(r chi.Router) {
	// 1. Serve raw OpenAPI spec in YAML and JSON formats
	r.Get("/swagger/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(OpenAPIYAML)
	})

	r.Get("/swagger/doc.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(SwaggerJSON)
	})

	r.Get("/swagger/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(SwaggerJSON)
	})

	// 2. Redirects for convenient access
	r.Get("/swagger", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})

	r.Get("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/swagger/index.html", http.StatusMovedPermanently)
	})

	// 3. Embedded Swagger UI via swaggo/http-swagger
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
		httpSwagger.DocExpansion("list"),
		httpSwagger.PersistAuthorization(true),
	))
}
