package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type releaseUpdateStatusResponse struct {
	Current       *releaseUpdateSummary `json:"current,omitempty"`
	Latest        *releaseUpdateSummary `json:"latest,omitempty"`
	State         string                `json:"state"`
	UpdateAllowed bool                  `json:"updateAllowed"`
	ActiveJob     *Job                  `json:"activeJob,omitempty"`
}

type releaseUpdateSummary struct {
	ReleaseID  string                                 `json:"releaseID"`
	Channel    string                                 `json:"channel"`
	CreatedAt  string                                 `json:"createdAt"`
	Components map[string]releaseUpdateComponentBrief `json:"components,omitempty"`
}

type releaseUpdateComponentBrief struct {
	Revision string `json:"revision"`
	SHA256   string `json:"sha256,omitempty"`
}

type releaseUpdateApplyRequest struct {
	fleetSignedRequest
}

type releaseComponentProvider func(context.Context, *releaseset.Manifest, string) error

func (service *Service) writeReleaseUpdateStatus(responseWriter http.ResponseWriter, request *http.Request) {
	latest, _ := service.fetchLatestReleaseManifest(request.Context())
	current := service.readCurrentReleaseManifest()
	activeJob := service.activeReleaseUpdateJob()
	response := releaseUpdateStatusResponse{
		Current:       releaseUpdateSummaryFromManifest(current),
		Latest:        releaseUpdateSummaryFromManifest(latest),
		State:         releaseUpdateState(current, latest, activeJob),
		UpdateAllowed: latest != nil && (current == nil || current.ReleaseID != latest.ReleaseID) && activeJob == nil,
		ActiveJob:     activeJob,
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) applyReleaseUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	service.startReleaseUpdate(responseWriter, request)
}

func (service *Service) applyReleaseUpdateSigned(responseWriter http.ResponseWriter, request *http.Request) {
	var payload releaseUpdateApplyRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.validateFleetSignedRequest(payload.fleetSignedRequest, isAllowedReleaseUpdateSignedAction); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	service.startReleaseUpdate(responseWriter, request)
}

func isAllowedReleaseUpdateSignedAction(action string) bool {
	return action == "release-update-apply"
}

func (service *Service) startReleaseUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	if job := service.activeReleaseUpdateJob(); job != nil {
		service.writeJSON(responseWriter, job)
		return
	}
	manifest, errorValue := service.fetchLatestReleaseManifest(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	current := service.readCurrentReleaseManifest()
	if current != nil && current.ReleaseID == manifest.ReleaseID {
		job := service.newJob("release-update")
		service.finishReleaseUpdateJob(job.JobID, "already_current", manifest)
		service.writeJSON(responseWriter, job)
		return
	}
	job := service.newJob("release-update")
	service.writeJSON(responseWriter, job)
	go service.runReleaseUpdateJob(context.Background(), job.JobID, manifest)
}

func (service *Service) rollbackReleaseUpdate(responseWriter http.ResponseWriter, _ *http.Request) {
	http.Error(responseWriter, "release rollback is not available in v1", http.StatusNotImplemented)
}

func (service *Service) runReleaseUpdateJob(ctx context.Context, jobID string, manifest *releaseset.Manifest) {
	service.runReleaseUpdateJobWithProvider(ctx, jobID, manifest, service.downloadReleaseComponents)
}

func (service *Service) runReleaseUpdateJobWithProvider(ctx context.Context, jobID string, manifest *releaseset.Manifest, provider releaseComponentProvider) {
	service.updateJob(jobID, "running", "verifying", "")
	if errorValue := manifest.Validate(); errorValue != nil {
		service.updateJob(jobID, "failed", "verifying", errorValue.Error())
		return
	}
	stagingPath := filepath.Join(service.Configuration.StateDirectory, "release-updates", "staging", jobID)
	_ = os.RemoveAll(stagingPath)
	if errorValue := os.MkdirAll(stagingPath, 0o700); errorValue != nil {
		service.updateJob(jobID, "failed", "staging", errorValue.Error())
		return
	}
	service.updateJob(jobID, "running", "downloading", "")
	if errorValue := provider(ctx, manifest, stagingPath); errorValue != nil {
		service.updateJob(jobID, "failed", "downloading", errorValue.Error())
		return
	}
	service.updateJob(jobID, "running", "installing", "")
	if errorValue := service.installReleaseComponents(ctx, jobID, manifest, stagingPath); errorValue != nil {
		service.updateJob(jobID, "failed", "installing", errorValue.Error())
		return
	}
	if errorValue := service.writeCurrentReleaseManifest(manifest); errorValue != nil {
		service.updateJob(jobID, "failed", "installing", errorValue.Error())
		return
	}
	service.finishReleaseUpdateJob(jobID, "completed", manifest)
	service.restartAdmindAfterReleaseUpdate(ctx, manifest)
}

func (service *Service) downloadReleaseComponents(ctx context.Context, manifest *releaseset.Manifest, stagingPath string) error {
	for componentName, component := range manifest.Components {
		componentStagingPath := filepath.Join(stagingPath, componentName)
		archivePath := filepath.Join(componentStagingPath, "component.tar.gz")
		if errorValue := os.MkdirAll(componentStagingPath, 0o700); errorValue != nil {
			return errorValue
		}
		if errorValue := service.downloadReleaseBlob(ctx, component, archivePath); errorValue != nil {
			return errorValue
		}
		if actualSHA256 := fileSHA256(archivePath); actualSHA256 != component.SHA256 {
			return fmt.Errorf("release component %s checksum mismatch", componentName)
		}
		if errorValue := extractTarGzip(archivePath, componentStagingPath); errorValue != nil {
			return fmt.Errorf("extract release component %s: %w", componentName, errorValue)
		}
	}
	return nil
}

func (service *Service) downloadReleaseBlob(ctx context.Context, component releaseset.Component, archivePath string) error {
	downloadURL := service.releaseRegistryURL(component.BlobPath)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if errorValue != nil {
		return errorValue
	}
	service.addReleaseDownloadHeaders(request)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("download release component %s: HTTP %d %s", component.Name, response.StatusCode, strings.TrimSpace(string(document)))
	}
	file, errorValue := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(file, response.Body)
	closeErrorValue := file.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}

func (service *Service) installReleaseComponents(ctx context.Context, jobID string, manifest *releaseset.Manifest, stagingPath string) error {
	if errorValue := service.installReleaseBinary(stagingPath, "internkim", "/usr/local/bin/internkim"); errorValue != nil {
		return errorValue
	}
	if errorValue := service.installReleaseBinary(stagingPath, "capabilityd", blueclawruntime.CapabilitydBinaryPath); errorValue != nil {
		return errorValue
	}
	if errorValue := service.installReleaseWeb(stagingPath); errorValue != nil {
		return errorValue
	}
	if errorValue := service.installReleaseSkills(stagingPath); errorValue != nil {
		return errorValue
	}
	if errorValue := service.installReleaseBlueclawPayload(ctx, jobID, stagingPath); errorValue != nil {
		return errorValue
	}
	if errorValue := service.installReleaseBinary(stagingPath, "admind", blueclawruntime.AdmindBinaryPath); errorValue != nil {
		return errorValue
	}
	if _, hasCapabilityd := manifest.Components["capabilityd"]; hasCapabilityd {
		if errorValue := service.restartReleaseCapabilitydServices(ctx); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) restartReleaseCapabilitydServices(ctx context.Context) error {
	for _, serviceName := range releaseCapabilitydServiceNames(blueclawUpdateTenantBasePath) {
		if output, errorValue := service.runCommand(ctx, "systemctl", "restart", serviceName); errorValue != nil {
			return fmt.Errorf("restart capabilityd %s: %s: %w", serviceName, strings.TrimSpace(string(output)), errorValue)
		}
	}
	return nil
}

func releaseCapabilitydServiceNames(tenantBasePath string) []string {
	return releaseTenantServiceNames(tenantBasePath, "internkim-tenant-capabilityd-", blueclawruntime.CapabilitydServiceName)
}

func releaseAdmindServiceNames(tenantBasePath string) []string {
	return releaseTenantServiceNames(tenantBasePath, "internkim-tenant-admind-", blueclawruntime.AdmindServiceName)
}

func releaseTenantServiceNames(tenantBasePath string, tenantServicePrefix string, fallbackServiceName string) []string {
	tenantIDs := releaseTenantIDs(tenantBasePath)
	if len(tenantIDs) == 0 {
		return []string{fallbackServiceName}
	}
	serviceNames := make([]string, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		serviceNames = append(serviceNames, tenantServicePrefix+tenantID+".service")
	}
	return serviceNames
}

func releaseTenantIDs(tenantBasePath string) []string {
	runtimeConfigurationPaths, errorValue := filepath.Glob(filepath.Join(tenantBasePath, "*", "blueclaw", "config", "runtime.json"))
	if errorValue != nil {
		return nil
	}
	tenantIDs := make([]string, 0, len(runtimeConfigurationPaths))
	for _, runtimeConfigurationPath := range runtimeConfigurationPaths {
		tenantID := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(runtimeConfigurationPath))))
		if strings.TrimSpace(tenantID) != "" {
			tenantIDs = append(tenantIDs, tenantID)
		}
	}
	return tenantIDs
}

func (service *Service) installReleaseBinary(stagingPath string, componentName string, targetPath string) error {
	componentRoot, errorValue := releaseComponentRoot(filepath.Join(stagingPath, componentName))
	if errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			return nil
		}
		return errorValue
	}
	sourcePath := componentRoot
	if information, errorValue := os.Stat(componentRoot); errorValue == nil && information.IsDir() {
		sourcePath = filepath.Join(componentRoot, filepath.Base(targetPath))
	}
	if _, errorValue := os.Stat(sourcePath); errorValue != nil {
		sourcePath = filepath.Join(componentRoot, componentName)
	}
	if _, errorValue := os.Stat(sourcePath); errorValue != nil {
		return nil
	}
	temporaryPath := targetPath + ".new"
	if errorValue := copyFile(sourcePath, temporaryPath, 0o755); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, targetPath)
}

func (service *Service) installReleaseWeb(stagingPath string) error {
	componentRoot, errorValue := service.releaseWebComponentRoot(stagingPath)
	if errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			return nil
		}
		return errorValue
	}
	temporaryPath := service.Configuration.AdminUIPath + ".new"
	previousPath := service.Configuration.AdminUIPath + ".previous"
	_ = os.RemoveAll(temporaryPath)
	if errorValue := copyReleaseDirectory(componentRoot, temporaryPath, 0o755); errorValue != nil {
		return errorValue
	}
	_ = os.RemoveAll(previousPath)
	if _, errorValue := os.Stat(service.Configuration.AdminUIPath); errorValue == nil {
		if errorValue := os.Rename(service.Configuration.AdminUIPath, previousPath); errorValue != nil {
			return errorValue
		}
	}
	return os.Rename(temporaryPath, service.Configuration.AdminUIPath)
}

func (service *Service) releaseWebComponentRoot(stagingPath string) (string, error) {
	componentRoot, errorValue := releaseComponentRoot(filepath.Join(stagingPath, "web"))
	if errorValue == nil || !errors.Is(errorValue, os.ErrNotExist) {
		return componentRoot, errorValue
	}
	return releaseComponentRoot(filepath.Join(stagingPath, "adminWeb"))
}

func (service *Service) installReleaseSkills(stagingPath string) error {
	componentRoot, errorValue := releaseComponentRoot(filepath.Join(stagingPath, "skills"))
	if errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			return nil
		}
		return errorValue
	}
	targetPath := filepath.Join(service.Configuration.BlueclawWorkspacePath, "skills")
	temporaryPath := targetPath + ".new"
	_ = os.RemoveAll(temporaryPath)
	if errorValue := copyReleaseDirectory(componentRoot, temporaryPath, 0o755); errorValue != nil {
		return errorValue
	}
	_ = os.RemoveAll(targetPath)
	if errorValue := os.Rename(temporaryPath, targetPath); errorValue != nil {
		return errorValue
	}
	_, _ = service.runCommand(context.Background(), "chown", "-R", "blueclaw:blueclaw", targetPath)
	return nil
}

func (service *Service) installReleaseBlueclawPayload(ctx context.Context, jobID string, stagingPath string) error {
	componentRoot, errorValue := releaseComponentRoot(filepath.Join(stagingPath, "blueclawPayload"))
	if errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			return nil
		}
		return errorValue
	}
	return service.installBlueclawPayloadArtifact(ctx, jobID, componentRoot)
}

func releaseComponentRoot(componentPath string) (string, error) {
	entries, errorValue := os.ReadDir(componentPath)
	if errorValue != nil {
		return "", errorValue
	}
	for _, entry := range entries {
		if entry.Name() == "component.tar.gz" {
			continue
		}
		return filepath.Join(componentPath, entry.Name()), nil
	}
	return "", os.ErrNotExist
}

func copyReleaseDirectory(sourcePath string, targetPath string, directoryMode os.FileMode) error {
	return filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, errorValue error) error {
		if errorValue != nil {
			return errorValue
		}
		relativePath, errorValue := filepath.Rel(sourcePath, path)
		if errorValue != nil {
			return errorValue
		}
		destinationPath := filepath.Join(targetPath, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(destinationPath, directoryMode)
		}
		information, errorValue := entry.Info()
		if errorValue != nil {
			return errorValue
		}
		return copyFile(path, destinationPath, information.Mode()&0o777)
	})
}

func (service *Service) restartAdmindAfterReleaseUpdate(ctx context.Context, manifest *releaseset.Manifest) {
	if _, hasAdmind := manifest.Components["admind"]; !hasAdmind {
		return
	}
	go func() {
		time.Sleep(800 * time.Millisecond)
		for _, serviceName := range releaseAdmindServiceNames(blueclawUpdateTenantBasePath) {
			_, _ = service.runCommand(ctx, "systemctl", "restart", serviceName)
		}
	}()
}

func (service *Service) fetchLatestReleaseManifest(ctx context.Context) (*releaseset.Manifest, error) {
	pointer, errorValue := service.fetchReleaseStablePointer(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.fetchReleaseManifest(ctx, pointer.ManifestURL)
}

func (service *Service) fetchReleaseStablePointer(ctx context.Context) (releaseset.StablePointer, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, service.releaseRegistryURL("channels/stable.json"), nil)
	if errorValue != nil {
		return releaseset.StablePointer{}, errorValue
	}
	service.addReleaseDownloadHeaders(request)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return releaseset.StablePointer{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return releaseset.StablePointer{}, fmt.Errorf("fetch stable release: HTTP %d", response.StatusCode)
	}
	var pointer releaseset.StablePointer
	if errorValue := json.NewDecoder(response.Body).Decode(&pointer); errorValue != nil {
		return releaseset.StablePointer{}, errorValue
	}
	return pointer, nil
}

func (service *Service) fetchReleaseManifest(ctx context.Context, manifestURL string) (*releaseset.Manifest, error) {
	if strings.TrimSpace(manifestURL) == "" {
		return nil, errors.New("release manifest URL is empty")
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, manifestURL, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	service.addReleaseDownloadHeaders(request)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch release manifest: HTTP %d", response.StatusCode)
	}
	var manifest releaseset.Manifest
	if errorValue := json.NewDecoder(response.Body).Decode(&manifest); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := manifest.VerifySignature(service.releaseSigningKey()); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := manifest.Validate(); errorValue != nil {
		return nil, errorValue
	}
	return &manifest, nil
}

func (service *Service) releaseRegistryURL(relativePath string) string {
	return strings.TrimRight(service.Configuration.ReleaseRegistryURL, "/") + "/" + strings.TrimLeft(relativePath, "/")
}

func (service *Service) addReleaseDownloadHeaders(request *http.Request) {
	token := strings.TrimSpace(readTrimmedFile(service.Configuration.ReleaseDownloadTokenPath))
	if token == "" {
		return
	}
	request.Header.Set("X-InternKim-Release-Token", token)
}

func (service *Service) releaseSigningKey() string {
	document, errorValue := os.ReadFile(service.Configuration.ReleaseSigningKeyPath)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func (service *Service) readCurrentReleaseManifest() *releaseset.Manifest {
	document, errorValue := os.ReadFile(service.currentReleaseManifestPath())
	if errorValue != nil {
		return nil
	}
	var manifest releaseset.Manifest
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return nil
	}
	return &manifest
}

func (service *Service) writeCurrentReleaseManifest(manifest *releaseset.Manifest) error {
	document, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(service.currentReleaseManifestPath()), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(service.currentReleaseManifestPath(), append(document, '\n'), 0o600)
}

func (service *Service) currentReleaseManifestPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "release-updates", "current.json")
}

func releaseUpdateSummaryFromManifest(manifest *releaseset.Manifest) *releaseUpdateSummary {
	if manifest == nil {
		return nil
	}
	components := map[string]releaseUpdateComponentBrief{}
	for componentName, component := range manifest.Components {
		components[componentName] = releaseUpdateComponentBrief{
			Revision: component.Revision,
			SHA256:   component.SHA256,
		}
	}
	return &releaseUpdateSummary{
		ReleaseID:  manifest.ReleaseID,
		Channel:    manifest.Channel,
		CreatedAt:  manifest.CreatedAt,
		Components: components,
	}
}

func releaseUpdateState(current *releaseset.Manifest, latest *releaseset.Manifest, activeJob *Job) string {
	if activeJob != nil {
		return "updating"
	}
	if latest == nil {
		return "unknown"
	}
	if current == nil || current.ReleaseID != latest.ReleaseID {
		return "update_available"
	}
	return "current"
}

func (service *Service) activeReleaseUpdateJob() *Job {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, job := range service.jobs {
		if job.Type == "release-update" && (job.Status == "pending" || job.Status == "running") {
			return job
		}
	}
	return nil
}

func (service *Service) finishReleaseUpdateJob(jobID string, status string, manifest *releaseset.Manifest) {
	service.mutex.Lock()
	job := service.jobs[jobID]
	if job != nil {
		job.Status = status
		job.Phase = status
		job.UpdatedAt = time.Now().UTC()
		job.Result = map[string]string{"releaseID": manifest.ReleaseID}
	}
	service.mutex.Unlock()
}
