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
		resolvedSourcePaths := asset.SourcePaths("/repo")
		if len(resolvedSourcePaths) == 0 {
			testInstance.Errorf("asset %q resolved to no source path", asset.Name)
		}
		for _, resolvedSourcePath := range resolvedSourcePaths {
			if !strings.HasPrefix(resolvedSourcePath, "/repo") {
				testInstance.Errorf("asset %q SourcePaths(\"/repo\") = %q, want prefix /repo", asset.Name, resolvedSourcePath)
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
