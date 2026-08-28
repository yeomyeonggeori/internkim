package deviceassets

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAllAssetsHaveRequiredFields(testInstance *testing.T) {
	repositoryRoot, errorValue := filepath.Abs(filepath.Join("..", ".."))
	if errorValue != nil {
		testInstance.Fatal(errorValue)
	}
	for _, asset := range All() {
		if asset.Name == "" {
			testInstance.Errorf("asset has empty Name: %+v", asset)
		}
		if asset.DevicePath == "" {
			testInstance.Errorf("asset %q has empty DevicePath", asset.Name)
		}
		if asset.SourcePaths == nil {
			testInstance.Errorf("asset %q has nil SourcePaths", asset.Name)
			continue
		}
		resolvedSourcePaths, errorValue := asset.SourcePaths(repositoryRoot)
		if errorValue != nil {
			testInstance.Errorf("asset %q SourcePaths: %v", asset.Name, errorValue)
			continue
		}
		if len(resolvedSourcePaths) == 0 {
			testInstance.Errorf("asset %q resolved to no source path", asset.Name)
		}
		for _, resolvedSourcePath := range resolvedSourcePaths {
			if !strings.HasPrefix(resolvedSourcePath, repositoryRoot) {
				testInstance.Errorf("asset %q SourcePaths(%q) = %q, want a path inside the repository", asset.Name, repositoryRoot, resolvedSourcePath)
			}
		}
	}
}

func TestAllAssetNamesAreUnique(testInstance *testing.T) {
	seenNames := map[string]bool{}
	for _, asset := range All() {
		if seenNames[asset.Name] {
			testInstance.Errorf("duplicate asset name: %q", asset.Name)
		}
		seenNames[asset.Name] = true
	}
}

// The skills asset enumerates per skill so the audience filter applies to the
// OTA blob and the SSH install alike; a per-skill list only lands correctly
// nested under its own name.
func TestSkillsAssetNestsItsSourcesByName(testInstance *testing.T) {
	asset, found := Find("skills")
	if !found {
		testInstance.Fatal("expected a skills asset")
	}
	if !asset.NestSourcesByName {
		testInstance.Fatal("the skills asset ships per-skill directories and must nest them by name")
	}
}
