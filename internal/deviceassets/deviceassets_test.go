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
		if asset.SourcePath == nil {
			testInstance.Errorf("asset %q has nil SourcePath", asset.Name)
			continue
		}
		resolvedSourcePath := asset.SourcePath("/repo")
		if !strings.HasPrefix(resolvedSourcePath, "/repo") {
			testInstance.Errorf("asset %q SourcePath(\"/repo\") = %q, want prefix /repo", asset.Name, resolvedSourcePath)
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
