package deviceassets

import (
	"path/filepath"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type DeviceKind string

const (
	DeviceKindWorkspace DeviceKind = "workspace"
	DeviceKindHost      DeviceKind = "host"
)

type Asset struct {
	Name       string
	SourcePath func(repositoryRoot string) string
	DevicePath string
	DeviceKind DeviceKind
}

func All() []Asset {
	return []Asset{
		{
			Name:       "skills",
			SourcePath: blueclawworkspace.SkillsPath,
			DevicePath: filepath.Join(blueclaw.BlueclawWorkspacePath, "skills"),
			DeviceKind: DeviceKindWorkspace,
		},
		{
			Name:       "tools",
			SourcePath: blueclawworkspace.ToolsPath,
			DevicePath: filepath.Join(blueclaw.BlueclawWorkspacePath, "tools"),
			DeviceKind: DeviceKindWorkspace,
		},
		{
			Name:       "fonts",
			SourcePath: fontsSourcePath,
			DevicePath: "/opt/internkim/fonts",
			DeviceKind: DeviceKindHost,
		},
	}
}

func Find(name string) (Asset, bool) {
	for _, asset := range All() {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

func fontsSourcePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, "assets", "fonts", "files")
}
