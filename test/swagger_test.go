package test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deb151292/neurealm-go-Assignment-/docs"
	"github.com/gin-gonic/gin"
)

func TestSwagger_Endpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	docs.RegisterSwaggerRoutes(r)

	// 1. Test /swagger/doc.json
	reqJSON := httptest.NewRequest("GET", "/swagger/doc.json", nil)
	recJSON := httptest.NewRecorder()
	r.ServeHTTP(recJSON, reqJSON)

	if recJSON.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /swagger/doc.json, got %d", recJSON.Code)
	}

	var jsonSpec map[string]interface{}
	if err := json.Unmarshal(recJSON.Body.Bytes(), &jsonSpec); err != nil {
		t.Fatalf("failed to parse /swagger/doc.json as valid JSON: %v", err)
	}
	if jsonSpec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi 3.0.3, got %v", jsonSpec["openapi"])
	}

	// 2. Test /swagger/openapi.yaml
	reqYAML := httptest.NewRequest("GET", "/swagger/openapi.yaml", nil)
	recYAML := httptest.NewRecorder()
	r.ServeHTTP(recYAML, reqYAML)

	if recYAML.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /swagger/openapi.yaml, got %d", recYAML.Code)
	}
	if !strings.Contains(recYAML.Body.String(), "openapi: 3.0.3") {
		t.Errorf("expected openapi.yaml to contain 'openapi: 3.0.3'")
	}

	// 3. Test /swagger/index.html
	reqUI := httptest.NewRequest("GET", "/swagger/index.html", nil)
	recUI := httptest.NewRecorder()
	r.ServeHTTP(recUI, reqUI)

	if recUI.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /swagger/index.html, got %d", recUI.Code)
	}
	if !strings.Contains(recUI.Body.String(), "swagger-ui") {
		t.Errorf("expected Swagger UI HTML page, got %s", recUI.Body.String())
	}

	// 4. Test Redirects: /swagger -> /swagger/index.html
	reqRedirect := httptest.NewRequest("GET", "/swagger", nil)
	recRedirect := httptest.NewRecorder()
	r.ServeHTTP(recRedirect, reqRedirect)

	if recRedirect.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 Moved Permanently for /swagger, got %d", recRedirect.Code)
	}
	if loc := recRedirect.Header().Get("Location"); loc != "/swagger/index.html" {
		t.Errorf("expected redirect to /swagger/index.html, got %s", loc)
	}

	// 5. Test Redirects: /docs -> /swagger/index.html
	reqDocs := httptest.NewRequest("GET", "/docs", nil)
	recDocs := httptest.NewRecorder()
	r.ServeHTTP(recDocs, reqDocs)

	if recDocs.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301 Moved Permanently for /docs, got %d", recDocs.Code)
	}
	if loc := recDocs.Header().Get("Location"); loc != "/swagger/index.html" {
		t.Errorf("expected redirect to /swagger/index.html, got %s", loc)
	}
}
