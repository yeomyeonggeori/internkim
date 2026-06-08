package tenantruntime

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	BasePath                   string
	SystemdNspawnDirectoryPath string
	SystemdSystemDirectoryPath string
	CloudflaredPath            string
	CommandRunner              CommandRunner
	OpenRouterKeyProvisioner   OpenRouterKeyProvisioner
}

type TenantStatus struct {
	Manifest               Manifest `json:"manifest"`
	ManifestPath           string   `json:"manifestPath"`
	ContainerUnitPath      string   `json:"containerUnitPath,omitempty"`
	ContainerRootReady     bool     `json:"containerRootReady"`
	ContainerRootBootable  bool     `json:"containerRootBootable"`
	ContainerUnitInstalled bool     `json:"containerUnitInstalled"`
	SecretsReady           bool     `json:"secretsReady"`
	WorkspaceReady         bool     `json:"workspaceReady"`
	BackupReady            bool     `json:"backupReady"`
}

func (service Service) CreateTenant(manifest Manifest) (TenantStatus, error) {
	if errorValue := manifest.Validate(); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := createRuntimeDirectories(paths); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := writeManifest(service.manifestPath(manifest.TenantID), manifest); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	return service.Status(manifest.TenantID)
}

func (service Service) Status(tenantID string) (TenantStatus, error) {
	manifest, errorValue := service.ReadManifest(tenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	return TenantStatus{
		Manifest:               manifest,
		ManifestPath:           service.manifestPath(manifest.TenantID),
		ContainerUnitPath:      service.containerUnitPath(manifest),
		ContainerRootReady:     directoryExists(paths.ContainerRootPath),
		ContainerRootBootable:  bootableRootFilesystemExists(paths.ContainerRootPath),
		ContainerUnitInstalled: fileExists(service.containerUnitPath(manifest)),
		SecretsReady:           directoryExists(paths.InternKimSecretsPath),
		WorkspaceReady:         directoryExists(paths.BlueclawWorkspacePath),
		BackupReady:            directoryExists(paths.BackupPath),
	}, nil
}

func (service Service) ReadManifest(tenantID string) (Manifest, error) {
	manifestPath := service.manifestPath(tenantID)
	document, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	var manifest Manifest
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return Manifest{}, errorValue
	}
	if errorValue := manifest.Validate(); errorValue != nil {
		return Manifest{}, errorValue
	}
	return manifest, nil
}

func (service Service) manifestPath(tenantID string) string {
	normalizedTenantID := strings.TrimSpace(strings.ToLower(tenantID))
	return filepath.Join(filepath.Clean(service.BasePath), normalizedTenantID, "manifest.json")
}

func (service Service) containerUnitPath(manifest Manifest) string {
	if strings.TrimSpace(manifest.ContainerName) == "" {
		return ""
	}
	return filepath.Join(service.nspawnDirectoryPath(), manifest.ContainerName+".nspawn")
}

func (service Service) nspawnDirectoryPath() string {
	if strings.TrimSpace(service.SystemdNspawnDirectoryPath) != "" {
		return filepath.Clean(service.SystemdNspawnDirectoryPath)
	}
	return "/etc/systemd/nspawn"
}

func (service Service) systemdSystemDirectoryPath() string {
	if strings.TrimSpace(service.SystemdSystemDirectoryPath) != "" {
		return filepath.Clean(service.SystemdSystemDirectoryPath)
	}
	return "/etc/systemd/system"
}

func (service Service) cloudflaredPath() string {
	if strings.TrimSpace(service.CloudflaredPath) != "" {
		return filepath.Clean(service.CloudflaredPath)
	}
	return "/usr/local/bin/cloudflared"
}

func createRuntimeDirectories(paths RuntimePaths) error {
	for _, directory := range []struct {
		path string
		mode os.FileMode
	}{
		{path: paths.TenantRootPath, mode: 0o750},
		{path: paths.ContainerRootPath, mode: 0o755},
		{path: paths.InternKimPath, mode: 0o750},
		{path: paths.InternKimSecretsPath, mode: 0o700},
		{path: paths.BlueclawRootPath, mode: 0o750},
		{path: paths.BlueclawWorkspacePath, mode: 0o750},
		{path: paths.BackupPath, mode: 0o700},
	} {
		if errorValue := os.MkdirAll(directory.path, directory.mode); errorValue != nil {
			return errorValue
		}
		if errorValue := os.Chmod(directory.path, directory.mode); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func writeManifest(path string, manifest Manifest) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("manifest path is required")
	}
	document, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func directoryExists(path string) bool {
	info, errorValue := os.Stat(path)
	return errorValue == nil && info.IsDir()
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, errorValue := os.Stat(path)
	return errorValue == nil && !info.IsDir()
}
