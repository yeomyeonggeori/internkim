package deviceassets

import (
	"strings"
	"testing"
)

func TestAllAssetsHaveRequiredFields(testInstance *testing.T) {
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
		resolvedSourcePaths, errorValue := asset.SourcePaths("/repo")
		if errorValue != nil {
			continue
		}
		if len(resolvedSourcePaths) == 0 {
			testInstance.Errorf("asset %q resolved to no source path and reported no reason", asset.Name)
		}
		for _, resolvedSourcePath := range resolvedSourcePaths {
			if !strings.HasPrefix(resolvedSourcePath, "/repo") {
				testInstance.Errorf("asset %q SourcePaths(\"/repo\") = %q, want prefix /repo", asset.Name, resolvedSourcePath)
			}
		}
	}
}

// An asset whose sources are discovered says why it found none, because the
// alternative is a release that ships an empty directory and reports success.
func TestASkillAssetWithNoPluginReportsWhy(testInstance *testing.T) {
	asset, found := Find("skills")
	if !found {
		testInstance.Fatal("the skills asset is gone")
	}

	_, errorValue := asset.SourcePaths("/repo")

	if errorValue == nil {
		testInstance.Fatal("a root carrying no plugin must be an error, not an empty list")
	}
	if !strings.Contains(errorValue.Error(), "submodule") {
		testInstance.Fatalf("the message must name what is missing, got %v", errorValue)
	}
}
