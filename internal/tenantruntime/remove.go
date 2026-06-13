package tenantruntime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type TenantRemoveOptions struct {
	Force                  bool
	CloudflareAccountID    string
	CloudflareTunnelID     string
	CloudflareAPIToken     string
	CloudflareAPITokenPath string
	CloudflareAPIBaseURL   string
	PublicHostnameTemplate string
}

type TenantRemovalReport struct {
	TenantID         string                         `json:"tenantID"`
	ManifestPath     string                         `json:"manifestPath"`
	ManifestExisted  bool                           `json:"manifestExisted"`
	Force            bool                           `json:"force"`
	Units            []TenantRemovalUnitReport      `json:"units"`
	Files            []TenantRemovalPathReport      `json:"files"`
	Directories      []TenantRemovalPathReport      `json:"directories"`
	CloudflareTunnel *CloudflareTunnelRemovalStatus `json:"cloudflareTunnel,omitempty"`
}

type TenantRemovalUnitReport struct {
	Name                  string `json:"name"`
	Path                  string `json:"path,omitempty"`
	Stopped               bool   `json:"stopped"`
	StopSkippedReason     string `json:"stopSkippedReason,omitempty"`
	Disabled              bool   `json:"disabled"`
	DisableSkippedReason  string `json:"disableSkippedReason,omitempty"`
	UnitFileRemoved       bool   `json:"unitFileRemoved"`
	UnitFileSkippedReason string `json:"unitFileSkippedReason,omitempty"`
}

type TenantRemovalPathReport struct {
	Path          string `json:"path"`
	Removed       bool   `json:"removed"`
	SkippedReason string `json:"skippedReason,omitempty"`
}

func (service Service) RemoveTenant(ctx context.Context, tenantID string, options TenantRemoveOptions) (TenantRemovalReport, error) {
	manifest, manifestExisted, errorValue := service.removalManifest(tenantID, options.Force)
	if errorValue != nil {
		return TenantRemovalReport{}, errorValue
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		return TenantRemovalReport{}, errorValue
	}
	report := TenantRemovalReport{
		TenantID:        manifest.TenantID,
		ManifestPath:    service.manifestPath(manifest.TenantID),
		ManifestExisted: manifestExisted,
		Force:           options.Force,
	}
	report.Units = service.removeTenantUnits(ctx, manifest)
	report.Files = service.removeTenantFiles(manifest)
	if cloudflareTunnelRemovalRequested(options) {
		status, errorValue := service.removeTenantCloudflareIngress(ctx, manifest, options)
		if errorValue != nil {
			return report, errorValue
		}
		report.CloudflareTunnel = &status
	}
	report.Directories = append(report.Directories, removeTenantDirectory(paths.TenantRootPath))
	if errorValue := service.reloadSystemd(ctx); errorValue != nil {
		return report, errorValue
	}
	return report, nil
}

func (service Service) removalManifest(tenantID string, force bool) (Manifest, bool, error) {
	manifest, errorValue := service.ReadManifest(tenantID)
	if errorValue == nil {
		return manifest, true, nil
	}
	if !errors.Is(errorValue, os.ErrNotExist) {
		return Manifest{}, false, errorValue
	}
	if !force {
		return Manifest{}, false, errors.New("tenant manifest does not exist; use --force to remove already-missing tenant state")
	}
	manifest, errorValue = NewCloudSharedManifest(tenantID, "", "", "", "")
	if errorValue != nil {
		return Manifest{}, false, errorValue
	}
	return manifest, false, nil
}

func (service Service) removeTenantUnits(ctx context.Context, manifest Manifest) []TenantRemovalUnitReport {
	units := tenantRemovalUnits(service.systemdSystemDirectoryPath(), manifest)
	reports := make([]TenantRemovalUnitReport, 0, len(units))
	for _, unit := range units {
		report := service.removeTenantUnit(ctx, unit)
		reports = append(reports, report)
	}
	return reports
}

func (service Service) removeTenantUnit(ctx context.Context, unit tenantRemovalUnit) TenantRemovalUnitReport {
	report := TenantRemovalUnitReport{Name: unit.name, Path: unit.path}
	if _, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{ExecutableName: "systemctl", Arguments: []string{"stop", unit.name}}); errorValue != nil {
		report.StopSkippedReason = errorValue.Error()
	} else {
		report.Stopped = true
	}
	if _, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{ExecutableName: "systemctl", Arguments: []string{"disable", unit.name}}); errorValue != nil {
		report.DisableSkippedReason = errorValue.Error()
	} else {
		report.Disabled = true
	}
	if strings.TrimSpace(unit.path) == "" {
		report.UnitFileSkippedReason = "no unit file"
		return report
	}
	pathReport := removeTenantFile(unit.path)
	report.UnitFileRemoved = pathReport.Removed
	report.UnitFileSkippedReason = pathReport.SkippedReason
	return report
}

func (service Service) removeTenantFiles(manifest Manifest) []TenantRemovalPathReport {
	return []TenantRemovalPathReport{
		removeTenantFile(service.containerUnitPath(manifest)),
	}
}

func (service Service) reloadSystemd(ctx context.Context) error {
	_, errorValue := service.commandRunner().Run(ctx, ExecutableCommand{
		ExecutableName: "systemctl",
		Arguments:      []string{"daemon-reload"},
	})
	return errorValue
}

type tenantRemovalUnit struct {
	name string
	path string
}

func tenantRemovalUnits(systemdDirectoryPath string, manifest Manifest) []tenantRemovalUnit {
	return []tenantRemovalUnit{
		{name: hostRuntimeAdmindServiceName(manifest), path: filepath.Join(systemdDirectoryPath, hostRuntimeAdmindServiceName(manifest))},
		{name: hostRuntimeBlueclawServiceName(manifest), path: filepath.Join(systemdDirectoryPath, hostRuntimeBlueclawServiceName(manifest))},
		{name: hostRuntimeCapabilitydServiceName(manifest), path: filepath.Join(systemdDirectoryPath, hostRuntimeCapabilitydServiceName(manifest))},
		{name: hostRuntimeGraphitiServiceName(manifest), path: filepath.Join(systemdDirectoryPath, hostRuntimeGraphitiServiceName(manifest))},
		{name: hostRuntimeMattermostServiceName(manifest), path: filepath.Join(systemdDirectoryPath, hostRuntimeMattermostServiceName(manifest))},
		{name: mattermostTunnelServiceName(manifest.TenantID), path: filepath.Join(systemdDirectoryPath, mattermostTunnelServiceName(manifest.TenantID))},
		{name: systemdNspawnServiceName(manifest)},
	}
}

func hostRuntimeMattermostServiceName(manifest Manifest) string {
	return "internkim-mattermost-" + manifest.TenantID + ".service"
}

func removeTenantFile(path string) TenantRemovalPathReport {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return TenantRemovalPathReport{SkippedReason: "empty path"}
	}
	if !fileExists(cleanPath) {
		return TenantRemovalPathReport{Path: cleanPath, SkippedReason: "absent"}
	}
	if errorValue := os.Remove(cleanPath); errorValue != nil {
		return TenantRemovalPathReport{Path: cleanPath, SkippedReason: errorValue.Error()}
	}
	return TenantRemovalPathReport{Path: cleanPath, Removed: true}
}

func removeTenantDirectory(path string) TenantRemovalPathReport {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return TenantRemovalPathReport{SkippedReason: "empty path"}
	}
	if !directoryExists(cleanPath) {
		return TenantRemovalPathReport{Path: cleanPath, SkippedReason: "absent"}
	}
	if errorValue := os.RemoveAll(cleanPath); errorValue != nil {
		return TenantRemovalPathReport{Path: cleanPath, SkippedReason: errorValue.Error()}
	}
	return TenantRemovalPathReport{Path: cleanPath, Removed: true}
}

func cloudflareTunnelRemovalRequested(options TenantRemoveOptions) bool {
	return strings.TrimSpace(options.CloudflareAccountID) != "" ||
		strings.TrimSpace(options.CloudflareTunnelID) != "" ||
		strings.TrimSpace(options.CloudflareAPIToken) != "" ||
		strings.TrimSpace(options.CloudflareAPITokenPath) != "" ||
		strings.TrimSpace(options.PublicHostnameTemplate) != ""
}

func (service Service) removeTenantCloudflareIngress(ctx context.Context, manifest Manifest, options TenantRemoveOptions) (CloudflareTunnelRemovalStatus, error) {
	return service.RemoveCloudflareTenantTunnelIngress(ctx, CloudflareTunnelRemoveOptions{
		AccountID:              options.CloudflareAccountID,
		TunnelID:               options.CloudflareTunnelID,
		APIToken:               options.CloudflareAPIToken,
		APITokenPath:           options.CloudflareAPITokenPath,
		APIBaseURL:             options.CloudflareAPIBaseURL,
		PublicHostnameTemplate: options.PublicHostnameTemplate,
		Manifests:              []Manifest{manifest},
	})
}
