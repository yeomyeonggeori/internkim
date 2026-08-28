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
	Name        string
	SourcePaths func(repositoryRoot string) ([]string, error)
	// A source path usually pours its contents into DevicePath. A nested
	// asset instead lands each source under DevicePath/<its base name>, which
	// is what lets the skills asset enumerate per skill and leave the
	// client-audience ones out.
	NestSourcesByName bool
	DevicePath        string
	DeviceKind        DeviceKind
}

func All() []Asset {
	return []Asset{
		{
			Name:              "skills",
			SourcePaths:       agentSkillSourcePaths,
			NestSourcesByName: true,
			DevicePath:        filepath.Join(blueclaw.BlueclawWorkspacePath, "skills"),
			DeviceKind:        DeviceKindWorkspace,
		},
		{
			Name:        "tools",
			SourcePaths: singleSourcePath(blueclawworkspace.ToolsPath),
			DevicePath:  filepath.Join(blueclaw.BlueclawWorkspacePath, "tools"),
			DeviceKind:  DeviceKindWorkspace,
		},
		{
			Name:        "fonts",
			SourcePaths: singleSourcePath(fontsSourcePath),
			DevicePath:  "/opt/internkim/fonts",
			DeviceKind:  DeviceKindHost,
		},
		{
			Name:        "document-conversion",
			SourcePaths: singleSourcePath(documentConversionSourcePath),
			DevicePath:  DocumentConversionDevicePath,
			DeviceKind:  DeviceKindHost,
		},
	}
}

const DocumentConversionDevicePath = "/opt/internkim/document-conversion"

func Find(name string) (Asset, bool) {
	for _, asset := range All() {
		if asset.Name == name {
			return asset, true
		}
	}
	return Asset{}, false
}

func agentSkillSourcePaths(repositoryRoot string) ([]string, error) {
	skillDirectories, errorValue := blueclawworkspace.SkillDirectories(repositoryRoot)
	if errorValue != nil {
		return nil, errorValue
	}
	sourcePaths := make([]string, 0, len(skillDirectories))
	for _, skillDirectory := range skillDirectories {
		sourcePaths = append(sourcePaths, skillDirectory.Path)
	}
	return sourcePaths, nil
}

func singleSourcePath(sourcePath func(repositoryRoot string) string) func(repositoryRoot string) ([]string, error) {
	return func(repositoryRoot string) ([]string, error) {
		return []string{sourcePath(repositoryRoot)}, nil
	}
}

func fontsSourcePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, "assets", "fonts", "files")
}

func documentConversionSourcePath(repositoryRoot string) string {
	return filepath.Join(repositoryRoot, "assets", "document-conversion")
}
