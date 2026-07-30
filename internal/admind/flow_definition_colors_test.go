package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestFlowDefinitionColorsPersist(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	ctx := context.Background()
	definitions, errorValue := service.readFlowDefinitions(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	definitions.Categories = []string{"여명거리", "김인턴"}
	definitions.CategoryColors = map[string]string{"여명거리": "#db2777"}
	definitions.Types = []string{"기능"}
	definitions.TypeColors = map[string]string{"기능": "#0891b2"}
	definitions.Sizes = []flowSizeDefinition{sizeDefinition("T", 21, 64, "개발", "기타", "비고")}
	if errorValue := service.writeFlowDefinitions(ctx, definitions); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloaded, errorValue := service.readFlowDefinitions(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reloaded.CategoryColors["여명거리"] != "#db2777" {
		t.Fatalf("categoryColors = %#v", reloaded.CategoryColors)
	}
	if _, hasColor := reloaded.CategoryColors["김인턴"]; hasColor {
		t.Fatalf("categoryColors carried a color nobody set: %#v", reloaded.CategoryColors)
	}
	if reloaded.TypeColors["기능"] != "#0891b2" {
		t.Fatalf("typeColors = %#v", reloaded.TypeColors)
	}
}

func TestFlowDefinitionsAPIKeepsColors(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	request := httptest.NewRequest(http.MethodPut, "/flow/api/definitions", strings.NewReader(`{
		"categories": ["여명거리"],
		"categoryColors": {"여명거리": "#DB2777"},
		"types": ["기능"],
		"typeColors": {"기능": "#0891b2", "없는종류": "not-a-color"},
		"sizes": [{"name": "M", "distanceKm": 3, "maxHours": 8}]
	}`))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	stored, errorValue := service.readFlowDefinitions(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.CategoryColors["여명거리"] != "#db2777" {
		t.Fatalf("categoryColors = %#v", stored.CategoryColors)
	}
	if stored.TypeColors["기능"] != "#0891b2" {
		t.Fatalf("typeColors = %#v", stored.TypeColors)
	}
	if _, hasColor := stored.TypeColors["없는종류"]; hasColor {
		t.Fatalf("kept an invalid color: %#v", stored.TypeColors)
	}
}
