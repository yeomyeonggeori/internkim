package admind

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOrganizationPageServesRouteAssetsAndIndex(t *testing.T) {
	adminUIPath := t.TempDir()
	if errorValue := os.MkdirAll(filepath.Join(adminUIPath, "organization", "assets"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "organization", "index.html"), []byte("organization index"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "organization", "assets", "app.js"), []byte("organization asset"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com"), AdminUIPath: adminUIPath})

	redirectResponse := httptest.NewRecorder()
	service.router().ServeHTTP(redirectResponse, httptest.NewRequest(http.MethodGet, "/organization", nil))
	if redirectResponse.Code != http.StatusFound || redirectResponse.Header().Get("Location") != "/organization/" {
		t.Fatalf("redirect status = %d location = %q", redirectResponse.Code, redirectResponse.Header().Get("Location"))
	}

	indexResponse := httptest.NewRecorder()
	service.router().ServeHTTP(indexResponse, httptest.NewRequest(http.MethodGet, "/organization/", nil))
	if indexResponse.Code != http.StatusOK || indexResponse.Body.String() != "organization index" {
		t.Fatalf("index status = %d body = %q", indexResponse.Code, indexResponse.Body.String())
	}

	assetResponse := httptest.NewRecorder()
	service.router().ServeHTTP(assetResponse, httptest.NewRequest(http.MethodGet, "/organization/assets/app.js", nil))
	if assetResponse.Code != http.StatusOK || assetResponse.Body.String() != "organization asset" {
		t.Fatalf("asset status = %d body = %q", assetResponse.Code, assetResponse.Body.String())
	}
}
