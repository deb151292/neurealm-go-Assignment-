package docs

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

//go:embed openapi.yaml
var OpenAPIYAML []byte

//go:embed swagger.json
var SwaggerJSON []byte

// RegisterSwaggerRoutes mounts the Swagger UI and OpenAPI documentation endpoints onto the Gin engine
func RegisterSwaggerRoutes(r *gin.Engine) {
	swaggerHandler := ginSwagger.WrapHandler(
		swaggerFiles.NewHandler(),
		ginSwagger.URL("/swagger/doc.json"),
		ginSwagger.PersistAuthorization(true),
	)

	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	r.GET("/docs", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})

	r.GET("/swagger/*any", func(c *gin.Context) {
		path := c.Param("any")
		switch path {
		case "/openapi.yaml":
			c.Data(http.StatusOK, "application/yaml; charset=utf-8", OpenAPIYAML)
		case "/doc.json", "/swagger.json":
			c.Data(http.StatusOK, "application/json; charset=utf-8", SwaggerJSON)
		default:
			swaggerHandler(c)
		}
	})
}
