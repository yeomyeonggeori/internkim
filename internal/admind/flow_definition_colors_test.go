package admind

import (
	"context"
	"path/filepath"
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
