package admind

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const blueclawUpdateChunkSize = 4 << 20
const blueclawUpdateTenantBasePath = "/srv/internkim/tenants"
const legacyBlueclawGuestMigrationPath = "/workspace/.blueclaw/migrations"
const drainTimeoutDefault = 75 * time.Second
const blueclawTaskDrainPollInterval = 5 * time.Second

var blueclawPreStopActiveTaskStatuses = []string{
	"planned",
	"running",
	"waiting_approval",
	"waiting_user_input",
	"blocked",
}

type blueclawPayloadInstallTarget struct {
	Name                              string
	ServiceName                       string
	HostWorkspacePath                 string
	WorkspaceImagePath                string
	RuntimeConfigurationPath          string
	WorkspaceRuntimeConfigurationPath string
	PayloadManifestPath               string
}

type blueclawPayloadRuntimeConfiguration struct {
	Firecracker struct {
		HostWorkspacePath  string `json:"hostWorkspacePath"`
		WorkspaceImagePath string `json:"workspaceImagePath"`
	} `json:"firecracker"`
}

type blueclawTaskRunListItem struct {
	TaskRunID string `json:"taskRunID"`
	Status    string `json:"status"`
}

type blueclawQuiesceRequest struct {
	Enabled bool `json:"enabled"`
}

type blueclawQuiesceResponse struct {
	ActiveTaskCount int `json:"activeTaskCount"`
}

type blueclawPrepareShutdownResponse struct {
	InterruptedTaskCount int `json:"interruptedTaskCount"`
}

type BlueclawUpdateUpload struct {
	UploadID       string
	Token          string
	Version        string
	Filename       string
	Size           int64
	SHA256         string
	CreatedAt      time.Time
	DirectoryPath  string
	ReceivedChunks map[int]bool
}

type blueclawUpdateUploadCreateRequest struct {
	fleetSignedRequest
	Version  string `json:"version"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

type blueclawUpdateUploadCreateResponse struct {
	UploadID    string `json:"uploadID"`
	UploadToken string `json:"uploadToken"`
	ChunkSize   int    `json:"chunkSize"`
}

type blueclawUpdateUploadCompleteRequest struct {
	Chunks int    `json:"chunks"`
	SHA256 string `json:"sha256"`
	Apply  *bool  `json:"apply,omitempty"`
}

type blueclawUpdateStatusResponse struct {
	Current       *blueclawUpdateArtifactMetadata `json:"current,omitempty"`
	Latest        *blueclawUpdateArtifactMetadata `json:"latest,omitempty"`
	State         string                          `json:"state"`
	UpdateAllowed bool                            `json:"updateAllowed"`
	ActiveJob     *Job                            `json:"activeJob,omitempty"`
}

type blueclawUpdateArtifactMetadata struct {
	Component        string                                  `json:"component"`
	Version          string                                  `json:"version"`
	BlueclawRevision string                                  `json:"blueclawRevision"`
	SHA256           string                                  `json:"sha256"`
	ManifestSHA256   string                                  `json:"manifestSHA256"`
	DownloadURL      string                                  `json:"downloadURL,omitempty"`
	CreatedAt        string                                  `json:"createdAt"`
	Manifest         blueclawruntime.PayloadArtifactManifest `json:"manifest"`
	ArchivePath      string                                  `json:"archivePath,omitempty"`
}

func (service *Service) handleBlueclawUpdateUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	switch {
	case request.Method == http.MethodPost && path == "/updates/blueclaw/uploads":
		service.createBlueclawUpdateUpload(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/updates/blueclaw/uploads/") && strings.Contains(path, "/chunks/"):
		service.writeBlueclawUpdateUploadChunk(responseWriter, request, path)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/updates/blueclaw/uploads/") && strings.HasSuffix(path, "/complete"):
		service.completeBlueclawUpdateUpload(responseWriter, request, path)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) createBlueclawUpdateUpload(responseWriter http.ResponseWriter, request *http.Request) {
	var payload blueclawUpdateUploadCreateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.validateFleetSignedRequest(payload.fleetSignedRequest, isAllowedBlueclawUpdateSignedAction); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	if errorValue := validateBlueclawUpdateUploadRequest(payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	uploadID := randomHex(16)
	uploadToken := randomHex(24)
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "blueclaw-updates", "uploads", uploadID)
	if errorValue := os.MkdirAll(filepath.Join(directoryPath, "chunks"), 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.mutex.Lock()
	service.blueclawUpdateUploads[uploadID] = &BlueclawUpdateUpload{
		UploadID:       uploadID,
		Token:          uploadToken,
		Version:        strings.TrimSpace(payload.Version),
		Filename:       filepath.Base(payload.Filename),
		Size:           payload.Size,
		SHA256:         strings.ToLower(strings.TrimSpace(payload.SHA256)),
		CreatedAt:      time.Now().UTC(),
		DirectoryPath:  directoryPath,
		ReceivedChunks: map[int]bool{},
	}
	service.mutex.Unlock()
	service.writeJSON(responseWriter, blueclawUpdateUploadCreateResponse{UploadID: uploadID, UploadToken: uploadToken, ChunkSize: blueclawUpdateChunkSize})
}

func validateBlueclawUpdateUploadRequest(payload blueclawUpdateUploadCreateRequest) error {
	if payload.Size <= 0 {
		return errors.New("blueclaw update artifact size is required")
	}
	sha256Value := strings.TrimSpace(payload.SHA256)
	if len(sha256Value) != 64 {
		return errors.New("blueclaw update artifact sha256 is invalid")
	}
	if _, errorValue := hex.DecodeString(sha256Value); errorValue != nil {
		return errors.New("blueclaw update artifact sha256 is invalid")
	}
	return nil
}

func isAllowedBlueclawUpdateSignedAction(action string) bool {
	return action == "blueclaw-update-upload"
}

func (service *Service) writeBlueclawUpdateUploadChunk(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID, chunkIndex, isValid := parseBlueclawUpdateUploadChunkPath(path)
	if !isValid {
		http.NotFound(responseWriter, request)
		return
	}
	upload, isFound := service.findBlueclawUpdateUpload(uploadID)
	if !isFound || !blueclawUpdateUploadTokenMatches(request, upload) {
		http.Error(responseWriter, "upload token invalid", http.StatusForbidden)
		return
	}
	chunkPath := filepath.Join(upload.DirectoryPath, "chunks", strconv.Itoa(chunkIndex))
	if errorValue := writeLimitedRequestBody(chunkPath, request.Body, blueclawUpdateChunkSize); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.markBlueclawUpdateUploadChunk(uploadID, chunkIndex)
	service.writeJSON(responseWriter, map[string]string{"status": "ok"})
}

func (service *Service) completeBlueclawUpdateUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID := strings.TrimSuffix(strings.TrimPrefix(path, "/updates/blueclaw/uploads/"), "/complete")
	upload, isFound := service.findBlueclawUpdateUpload(uploadID)
	if !isFound || !blueclawUpdateUploadTokenMatches(request, upload) {
		http.Error(responseWriter, "upload token invalid", http.StatusForbidden)
		return
	}
	var payload blueclawUpdateUploadCompleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	archivePath := filepath.Join(upload.DirectoryPath, firstNonEmpty(upload.Filename, "blueclaw-payload.tar.gz"))
	if errorValue := assembleChunkDirectory(upload.DirectoryPath, payload.Chunks, archivePath, "blueclaw update upload"); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := validateBlueclawUpdateArchiveHash(archivePath, firstNonEmpty(payload.SHA256, upload.SHA256)); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	artifactPath, metadata, errorValue := service.prepareBlueclawUpdateArtifact(upload, archivePath)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if payload.Apply != nil && !*payload.Apply {
		service.writeJSON(responseWriter, metadata)
		return
	}
	job := service.newJob("blueclaw-update")
	service.runBlueclawUpdateJob(request.Context(), job.JobID, artifactPath, metadata)
	if completedJob, isFound := service.findJob(job.JobID); isFound {
		service.writeJSON(responseWriter, completedJob)
		return
	}
	service.writeJSON(responseWriter, job)
}

func (service *Service) writeBlueclawUpdateStatus(responseWriter http.ResponseWriter) {
	latest := service.readLatestBlueclawUpdateMetadata()
	current := service.readCurrentBlueclawUpdateMetadata()
	response := blueclawUpdateStatusResponse{
		Current:       publicBlueclawUpdateMetadata(current),
		Latest:        publicBlueclawUpdateMetadata(latest),
		State:         blueclawUpdateState(current, latest),
		UpdateAllowed: latest != nil,
		ActiveJob:     service.activeBlueclawUpdateJob(),
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) applyLatestBlueclawUpdate(responseWriter http.ResponseWriter, _ *http.Request) {
	if job := service.activeBlueclawUpdateJob(); job != nil {
		service.writeJSON(responseWriter, job)
		return
	}
	metadata := service.readLatestBlueclawUpdateMetadata()
	if metadata == nil || strings.TrimSpace(metadata.ArchivePath) == "" {
		http.Error(responseWriter, "no blueclaw update artifact is available", http.StatusBadRequest)
		return
	}
	artifactPath, errorValue := service.extractBlueclawUpdateArchive(metadata.ArchivePath)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	job := service.newJob("blueclaw-update")
	service.writeJSON(responseWriter, job)
	go service.runBlueclawUpdateJob(context.Background(), job.JobID, artifactPath, metadata)
}

func (service *Service) runBlueclawUpdateJob(ctx context.Context, jobID string, artifactPath string, metadata *blueclawUpdateArtifactMetadata) {
	service.updateJob(jobID, "running", "verifying", "")
	if _, errorValue := blueclawruntime.ValidatePayloadArtifactDirectory(artifactPath); errorValue != nil {
		service.updateJob(jobID, "failed", "verifying", errorValue.Error())
		return
	}
	if service.isBlueclawPayloadAlreadyCurrent(artifactPath) {
		service.finishBlueclawUpdateJob(jobID, "already_current", metadata)
		return
	}
	if errorValue := service.installBlueclawPayloadArtifact(ctx, jobID, artifactPath); errorValue != nil {
		_, _ = service.runCommand(ctx, "sh", "-lc", blueclawruntime.StartAfterPayloadSyncCommand())
		service.updateJob(jobID, "failed", "installing", errorValue.Error())
		return
	}
	service.finishBlueclawUpdateJob(jobID, "completed", metadata)
}

func (service *Service) finishBlueclawUpdateJob(jobID string, status string, metadata *blueclawUpdateArtifactMetadata) {
	service.mutex.Lock()
	job := service.jobs[jobID]
	if job != nil {
		job.Status = status
		job.Phase = status
		job.UpdatedAt = time.Now().UTC()
		job.Result = map[string]string{
			"blueclawRevision": metadata.BlueclawRevision,
			"version":          metadata.Version,
		}
	}
	service.mutex.Unlock()
}

func (service *Service) installBlueclawPayloadArtifact(ctx context.Context, jobID string, artifactPath string) error {
	targets := service.blueclawPayloadInstallTargets()
	if len(targets) == 0 {
		targets = []blueclawPayloadInstallTarget{canonicalBlueclawPayloadInstallTarget()}
	}
	for _, target := range targets {
		if service.tenantServiceIsDisabled(ctx, target.ServiceName) {
			continue
		}
		if errorValue := service.installBlueclawPayloadArtifactForTarget(ctx, jobID, artifactPath, target); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) installBlueclawPayloadArtifactForTarget(ctx context.Context, jobID string, artifactPath string, target blueclawPayloadInstallTarget) error {
	if errorValue := syncBlueclawRuntimeConfigurationForTarget(target); errorValue != nil {
		return fmt.Errorf("%s: sync blueclaw runtime configuration: %w", target.Name, errorValue)
	}
	service.updateJob(jobID, "running", "stopping", "")
	service.drainBlueclawTasksBeforeStop(ctx, target, drainTimeoutDefault)
	if output, errorValue := service.runCommand(ctx, "sh", "-lc", stopBlueclawPayloadTargetCommand(target)); errorValue != nil {
		return fmt.Errorf("%s: stop blueclaw before payload sync: %s: %w", target.Name, strings.TrimSpace(string(output)), errorValue)
	}
	if errorValue := service.repairWorkspaceImageIfUnhealthy(ctx, jobID, target); errorValue != nil {
		return errorValue
	}
	service.updateJob(jobID, "running", "installing", "")
	if output, errorValue := service.runCommand(ctx, "sh", "-lc", hostWorkspacePayloadSyncCommandForTarget(artifactPath, target)); errorValue != nil {
		return fmt.Errorf("%s: sync blueclaw payload host workspace: %s: %w", target.Name, strings.TrimSpace(string(output)), errorValue)
	}
	syncCommand := blueclawPayloadWorkspaceSyncCommand(target)
	if output, errorValue := service.runCommand(ctx, "sh", "-lc", syncCommand); errorValue != nil {
		_, _ = service.runCommand(ctx, "sh", "-lc", startBlueclawPayloadTargetCommand(target))
		return fmt.Errorf("%s: sync blueclaw payload workspace: %s: %w", target.Name, strings.TrimSpace(string(output)), errorValue)
	}
	if matches, mismatchDetail := service.blueclawWorkspaceManifestMatchesTarget(artifactPath, target); !matches {
		return fmt.Errorf("%s: sync blueclaw payload workspace: workspace manifest mismatch: %s", target.Name, mismatchDetail)
	}
	installManifestCommand := "install -m 0644 " + quoteBlueclawUpdateShellValue(filepath.Join(artifactPath, "manifest.json")) + " " + quoteBlueclawUpdateShellValue(target.PayloadManifestPath)
	if output, errorValue := service.runCommand(ctx, "sh", "-lc", installManifestCommand); errorValue != nil {
		return fmt.Errorf("%s: install blueclaw payload manifest: %s: %w", target.Name, strings.TrimSpace(string(output)), errorValue)
	}
	service.updateJob(jobID, "running", "restarting", "")
	if output, errorValue := service.runCommand(ctx, "sh", "-lc", startBlueclawPayloadTargetCommand(target)); errorValue != nil {
		return fmt.Errorf("%s: start blueclaw after payload sync: %s: %w", target.Name, strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}

func blueclawPayloadWorkspaceSyncCommand(target blueclawPayloadInstallTarget) string {
	return strings.Join([]string{
		blueclawruntime.BlueclawSupervisorBinaryPath,
		"sync-workspace",
		"--atomic",
		"--preserve-guest-state",
		"--workspace-image", quoteBlueclawUpdateShellValue(target.WorkspaceImagePath),
		"--source", quoteBlueclawUpdateShellValue(target.HostWorkspacePath),
	}, " ")
}

func (service *Service) drainBlueclawTasksBeforeStop(ctx context.Context, target blueclawPayloadInstallTarget, timeout time.Duration) {
	service.drainBlueclawTasksBeforeStopWithPollInterval(ctx, target, timeout, blueclawTaskDrainPollInterval)
}

func (service *Service) drainBlueclawTasksBeforeStopWithPollInterval(ctx context.Context, target blueclawPayloadInstallTarget, timeout time.Duration, pollInterval time.Duration) {
	if timeout <= 0 {
		timeout = drainTimeoutDefault
	}
	if pollInterval <= 0 {
		pollInterval = blueclawTaskDrainPollInterval
	}
	defer service.markBlueclawTasksForPlannedShutdown(ctx, target)
	service.engageBlueclawQuiesce(ctx, target)
	drainContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		activeTaskCount, errorValue := service.fetchActiveBlueclawTaskCount(drainContext)
		if errorValue != nil {
			if drainContext.Err() != nil {
				log.Printf("Blueclaw pre-stop drain for %s timed out after %s; proceeding", target.Name, timeout)
				return
			}
			log.Printf("Blueclaw pre-stop drain for %s skipped: task API unavailable: %v", target.Name, errorValue)
			return
		}
		if activeTaskCount == 0 {
			log.Printf("Blueclaw pre-stop drain for %s completed: no active tasks", target.Name)
			return
		}
		log.Printf("Blueclaw pre-stop drain for %s: waiting on %d active task(s)", target.Name, activeTaskCount)
		select {
		case <-time.After(pollInterval):
		case <-drainContext.Done():
			log.Printf("Blueclaw pre-stop drain for %s timed out after %s with %d active task(s); proceeding", target.Name, timeout, activeTaskCount)
			return
		}
	}
}

func (service *Service) markBlueclawTasksForPlannedShutdown(ctx context.Context, target blueclawPayloadInstallTarget) {
	response := blueclawPrepareShutdownResponse{}
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/runtime/prepare-shutdown", nil, &response); errorValue != nil {
		log.Printf("Blueclaw pre-stop for %s: prepare-shutdown unavailable: %v", target.Name, errorValue)
		return
	}
	log.Printf("Blueclaw pre-stop for %s: marked %d task(s) interrupted for planned shutdown", target.Name, response.InterruptedTaskCount)
}

func (service *Service) engageBlueclawQuiesce(ctx context.Context, target blueclawPayloadInstallTarget) {
	requestBody := blueclawQuiesceRequest{Enabled: true}
	response := blueclawQuiesceResponse{}
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/quiesce", requestBody, &response); errorValue != nil {
		log.Printf("Blueclaw pre-stop drain for %s: quiesce control unavailable, polling active tasks only: %v", target.Name, errorValue)
		return
	}
	log.Printf("Blueclaw pre-stop drain for %s: new-task intake quiesced (%d active task(s))", target.Name, response.ActiveTaskCount)
}

func (service *Service) fetchActiveBlueclawTaskCount(ctx context.Context) (int, error) {
	activeTaskCount := 0
	for _, status := range blueclawPreStopActiveTaskStatuses {
		taskRuns, errorValue := service.fetchBlueclawTaskRunsByStatus(ctx, status)
		if errorValue != nil {
			return 0, errorValue
		}
		activeTaskCount += countActiveBlueclawTaskRuns(taskRuns)
	}
	return activeTaskCount, nil
}

func (service *Service) fetchBlueclawTaskRunsByStatus(ctx context.Context, status string) ([]blueclawTaskRunListItem, error) {
	path := "/admin/api/task?status=" + url.QueryEscape(status)
	taskRuns := []blueclawTaskRunListItem{}
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, path, nil, &taskRuns); errorValue != nil {
		return nil, errorValue
	}
	return taskRuns, nil
}

func countActiveBlueclawTaskRuns(taskRuns []blueclawTaskRunListItem) int {
	activeTaskCount := 0
	for _, taskRun := range taskRuns {
		if isActiveBlueclawTaskStatus(taskRun.Status) {
			activeTaskCount++
		}
	}
	return activeTaskCount
}

func isActiveBlueclawTaskStatus(status string) bool {
	for _, activeStatus := range blueclawPreStopActiveTaskStatuses {
		if status == activeStatus {
			return true
		}
	}
	return false
}

func (service *Service) isBlueclawPayloadAlreadyCurrent(artifactPath string) bool {
	manifestDocument, errorValue := os.ReadFile(filepath.Join(artifactPath, "manifest.json"))
	if errorValue != nil {
		return false
	}
	targets := service.blueclawPayloadInstallTargets()
	if len(targets) == 0 {
		targets = []blueclawPayloadInstallTarget{canonicalBlueclawPayloadInstallTarget()}
	}
	for _, target := range targets {
		currentManifestDocument, errorValue := os.ReadFile(target.PayloadManifestPath)
		if errorValue != nil || string(manifestDocument) != string(currentManifestDocument) {
			return false
		}
		if matches, _ := service.blueclawWorkspaceManifestMatchesTarget(artifactPath, target); !matches {
			return false
		}
		if !isBlueclawRuntimeConfigurationCurrentForTarget(target) {
			return false
		}
	}
	return true
}

func (service *Service) blueclawWorkspaceManifestMatches(artifactPath string) bool {
	matches, _ := service.blueclawWorkspaceManifestMatchesTarget(artifactPath, canonicalBlueclawPayloadInstallTarget())
	return matches
}

func (service *Service) blueclawWorkspaceManifestMatchesTarget(artifactPath string, target blueclawPayloadInstallTarget) (bool, string) {
	manifestDocument, errorValue := os.ReadFile(filepath.Join(artifactPath, "manifest.json"))
	if errorValue != nil {
		return false, "artifact manifest unreadable: " + errorValue.Error()
	}
	command := "debugfs -R " + quoteBlueclawUpdateShellValue("cat /.blueclaw/runtime/current/manifest.json") + " " + quoteBlueclawUpdateShellValue(target.WorkspaceImagePath) + " 2>/dev/null || true"
	output, errorValue := service.runCommand(context.Background(), "sh", "-lc", command)
	if errorValue != nil {
		return false, "image manifest read failed: " + errorValue.Error()
	}
	if string(manifestDocument) == string(output) {
		return true, ""
	}
	diagnosticCommand := "debugfs -R " + quoteBlueclawUpdateShellValue("stat /.blueclaw/runtime/current") + " " + quoteBlueclawUpdateShellValue(target.WorkspaceImagePath) + " 2>&1 | head -3"
	diagnosticOutput, _ := service.runCommand(context.Background(), "sh", "-lc", diagnosticCommand)
	return false, fmt.Sprintf(
		"image manifest %d bytes %q vs artifact %d bytes %q; current stat: %s",
		len(output), truncateBlueclawUpdateDetail(string(output)),
		len(manifestDocument), truncateBlueclawUpdateDetail(string(manifestDocument)),
		truncateBlueclawUpdateDetail(string(diagnosticOutput)),
	)
}

func (service *Service) repairWorkspaceImageIfUnhealthy(ctx context.Context, jobID string, target blueclawPayloadInstallTarget) error {
	probeCommand := "debugfs -R " + quoteBlueclawUpdateShellValue("stat /") + " " + quoteBlueclawUpdateShellValue(target.WorkspaceImagePath) + " 2>&1"
	probeOutput, probeError := service.runCommand(ctx, "sh", "-lc", probeCommand)
	if probeError == nil && !workspaceImageProbeLooksCorrupted(string(probeOutput)) {
		return nil
	}
	if strings.Contains(strings.ToLower(string(probeOutput)), "no such file") {
		return nil
	}
	service.updateJob(jobID, "running", "repairing", "")
	repairCommand := workspaceImageRepairCommand(target)
	repairOutput, repairError := service.runCommand(ctx, "sh", "-lc", repairCommand)
	if repairError != nil {
		return fmt.Errorf("%s: repair blueclaw workspace image: %s: %w", target.Name, truncateBlueclawUpdateDetail(string(repairOutput)), repairError)
	}
	return nil
}

func workspaceImageRepairCommand(target blueclawPayloadInstallTarget) string {
	imagePath := quoteBlueclawUpdateShellValue(target.WorkspaceImagePath)
	hostPath := quoteBlueclawUpdateShellValue(target.HostWorkspacePath)
	capabilitydService := quoteBlueclawUpdateShellValue(blueclawruntime.CapabilitydServiceName)
	return strings.TrimSpace(fmt.Sprintf(`
image=%s
for loop_device in $(losetup -j "$image" 2>/dev/null | cut -d: -f1); do
  for mount_target in $(findmnt -rn -o TARGET -S "$loop_device" 2>/dev/null); do
    if ! umount "$mount_target" 2>/dev/null; then
      systemctl stop %s 2>/dev/null || true
      sleep 1
      if ! umount "$mount_target"; then
        echo "unmount blocked at $mount_target:"
        fuser -vm "$mount_target" 2>&1 | head -6
        systemctl start %s 2>/dev/null || true
        exit 3
      fi
    fi
  done
done
e2fsck -fy "$image"
repair_status=$?
if [ "$repair_status" -gt 2 ]; then
  echo '-- mounts:'
  mount | grep -i -e blueclaw -e workspace || true
  echo '-- losetup:'
  losetup -j "$image" || true
fi
if [ -n %s ]; then mkdir -p %s; fi
systemctl start %s 2>/dev/null || true
[ "$repair_status" -le 2 ]
`, imagePath, capabilitydService, capabilitydService, hostPath, hostPath, capabilitydService))
}

func workspaceImageProbeLooksCorrupted(probeOutput string) bool {
	lowered := strings.ToLower(probeOutput)
	for _, marker := range []string{"checksum", "corrupt", "bad magic", "filesystem not open", "can't read", "cannot read"} {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

func truncateBlueclawUpdateDetail(value string) string {
	compact := strings.Join(strings.Fields(value), " ")
	if len(compact) <= 160 {
		return compact
	}
	return compact[:160] + "..."
}

func (service *Service) blueclawPayloadInstallTargets() []blueclawPayloadInstallTarget {
	return blueclawPayloadInstallTargets(blueclawUpdateTenantBasePath)
}

func blueclawPayloadInstallTargets(tenantBasePath string) []blueclawPayloadInstallTarget {
	targets := []blueclawPayloadInstallTarget{canonicalBlueclawPayloadInstallTarget()}
	return append(targets, blueclawPayloadTenantInstallTargets(tenantBasePath)...)
}

func blueclawPayloadTenantInstallTargets(tenantBasePath string) []blueclawPayloadInstallTarget {
	runtimeConfigurationPaths, errorValue := filepath.Glob(filepath.Join(tenantBasePath, "*", "blueclaw", "config", "runtime.json"))
	if errorValue != nil {
		return nil
	}
	targets := []blueclawPayloadInstallTarget{}
	for _, runtimeConfigurationPath := range runtimeConfigurationPaths {
		if target, ok := blueclawPayloadTenantInstallTarget(runtimeConfigurationPath); ok {
			targets = append(targets, target)
		}
	}
	return targets
}

func blueclawPayloadTenantInstallTarget(runtimeConfigurationPath string) (blueclawPayloadInstallTarget, bool) {
	document, errorValue := os.ReadFile(runtimeConfigurationPath)
	if errorValue != nil {
		return blueclawPayloadInstallTarget{}, false
	}
	var runtimeConfiguration blueclawPayloadRuntimeConfiguration
	if errorValue := json.Unmarshal(document, &runtimeConfiguration); errorValue != nil {
		return blueclawPayloadInstallTarget{}, false
	}
	tenantID := filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(runtimeConfigurationPath))))
	hostWorkspacePath := strings.TrimSpace(runtimeConfiguration.Firecracker.HostWorkspacePath)
	workspaceImagePath := strings.TrimSpace(runtimeConfiguration.Firecracker.WorkspaceImagePath)
	if strings.TrimSpace(tenantID) == "" || hostWorkspacePath == "" || workspaceImagePath == "" {
		return blueclawPayloadInstallTarget{}, false
	}
	blueclawRootPath := filepath.Dir(filepath.Dir(runtimeConfigurationPath))
	return blueclawPayloadInstallTarget{
		Name:                              tenantID,
		ServiceName:                       "internkim-tenant-blueclaw-" + tenantID + ".service",
		HostWorkspacePath:                 hostWorkspacePath,
		WorkspaceImagePath:                workspaceImagePath,
		RuntimeConfigurationPath:          runtimeConfigurationPath,
		WorkspaceRuntimeConfigurationPath: filepath.Join(hostWorkspacePath, ".blueclaw", "config", "runtime.json"),
		PayloadManifestPath:               filepath.Join(blueclawRootPath, "payload-manifest.json"),
	}, true
}

func canonicalBlueclawPayloadInstallTarget() blueclawPayloadInstallTarget {
	return blueclawPayloadInstallTarget{
		Name:                              "blueclaw",
		ServiceName:                       blueclawruntime.BlueclawServiceName,
		HostWorkspacePath:                 blueclawruntime.BlueclawWorkspacePath,
		WorkspaceImagePath:                blueclawruntime.BlueclawWorkspaceImagePath,
		RuntimeConfigurationPath:          blueclawruntime.BlueclawRuntimeConfigPath,
		WorkspaceRuntimeConfigurationPath: filepath.Join(blueclawruntime.BlueclawWorkspacePath, ".blueclaw", "config", "runtime.json"),
		PayloadManifestPath:               blueclawruntime.BlueclawPayloadManifestPath,
	}
}

func syncBlueclawRuntimeConfigurationForTarget(target blueclawPayloadInstallTarget) error {
	for _, configurationPath := range blueclawRuntimeConfigurationPathsForTarget(target) {
		if errorValue := syncBlueclawRuntimeConfigurationPath(configurationPath); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func syncBlueclawRuntimeConfigurationPath(configurationPath string) error {
	if strings.TrimSpace(configurationPath) == "" {
		return nil
	}
	document, errorValue := os.ReadFile(configurationPath)
	if os.IsNotExist(errorValue) {
		return nil
	}
	if errorValue != nil {
		return errorValue
	}
	updatedDocument, errorValue := refreshedBlueclawRuntimeConfiguration(string(document))
	if errorValue != nil {
		return errorValue
	}
	if updatedDocument == string(document) {
		return nil
	}
	return os.WriteFile(configurationPath, []byte(updatedDocument), 0o640)
}

func refreshedBlueclawRuntimeConfiguration(document string) (string, error) {
	migratedDocument := strings.ReplaceAll(document, legacyBlueclawGuestMigrationPath, blueclawruntime.BlueclawGuestMigrationPath)
	return refreshBlueclawCapabilityContract(migratedDocument)
}

func refreshBlueclawCapabilityContract(document string) (string, error) {
	var runtimeDocument map[string]any
	if errorValue := json.Unmarshal([]byte(document), &runtimeDocument); errorValue != nil {
		return "", errorValue
	}
	contract := blueclawruntime.CurrentCapabilityContract()
	if capabilitiesSection, ok := runtimeDocument["capabilities"].(map[string]any); ok {
		delete(capabilitiesSection, "toolNames")
		capabilitiesSection["protocolVersion"] = contract.ProtocolVersion
		capabilitiesSection["aggregateProtocolHash"] = contract.AggregateProtocolHash
		capabilitiesSection["toolDescriptors"] = contract.ToolDescriptors
		if routing, ok := capabilitiesSection["routing"].(map[string]any); ok {
			routing["candidates"] = contract.RoutingCandidates
		}
	}
	refreshedBytes, errorValue := json.MarshalIndent(runtimeDocument, "", "  ")
	if errorValue != nil {
		return "", errorValue
	}
	return string(refreshedBytes) + "\n", nil
}

func isBlueclawRuntimeConfigurationCurrentForTarget(target blueclawPayloadInstallTarget) bool {
	for _, configurationPath := range blueclawRuntimeConfigurationPathsForTarget(target) {
		if !isBlueclawRuntimeConfigurationPathCurrent(configurationPath) {
			return false
		}
	}
	return true
}

func isBlueclawRuntimeConfigurationPathCurrent(configurationPath string) bool {
	if strings.TrimSpace(configurationPath) == "" {
		return true
	}
	document, errorValue := os.ReadFile(configurationPath)
	if os.IsNotExist(errorValue) {
		return true
	}
	if errorValue != nil {
		return false
	}
	refreshedDocument, errorValue := refreshedBlueclawRuntimeConfiguration(string(document))
	return errorValue == nil && refreshedDocument == string(document)
}

func blueclawRuntimeConfigurationPathsForTarget(target blueclawPayloadInstallTarget) []string {
	return []string{
		target.RuntimeConfigurationPath,
		target.WorkspaceRuntimeConfigurationPath,
	}
}

func hostWorkspacePayloadSyncCommandForTarget(artifactPath string, target blueclawPayloadInstallTarget) string {
	sourcePath := filepath.Join(artifactPath, "workspace", ".blueclaw", "runtime")
	targetPath := filepath.Join(target.HostWorkspacePath, ".blueclaw", "runtime")
	return strings.Join([]string{
		"mkdir -p", quoteBlueclawUpdateShellValue(filepath.Dir(targetPath)),
		"&& rsync -a --delete", quoteBlueclawUpdateShellValue(sourcePath + "/"), quoteBlueclawUpdateShellValue(targetPath + "/"),
		"&& chown -R blueclaw:blueclaw", quoteBlueclawUpdateShellValue(targetPath),
	}, " ")
}

func stopBlueclawPayloadTargetCommand(target blueclawPayloadInstallTarget) string {
	serviceName := quoteBlueclawUpdateShellValue(target.ServiceName)
	return `systemctl stop ` + serviceName + ` >/dev/null 2>&1 || true
for _ in $(seq 1 20); do
  if ! systemctl is-active --quiet ` + serviceName + `; then
    exit 0
  fi
  sleep 1
done
systemctl kill ` + serviceName + ` --kill-who=all --signal=KILL >/dev/null 2>&1 || true
for _ in $(seq 1 20); do
  if ! systemctl is-active --quiet ` + serviceName + `; then
    exit 0
  fi
  sleep 1
done
systemctl status ` + serviceName + ` --no-pager -l 2>/dev/null || true
exit 1`
}

func startBlueclawPayloadTargetCommand(target blueclawPayloadInstallTarget) string {
	serviceName := quoteBlueclawUpdateShellValue(target.ServiceName)
	return `systemctl start ` + serviceName + `
for _ in $(seq 1 20); do
  if systemctl is-active --quiet ` + serviceName + `; then
    exit 0
  fi
  sleep 1
done
systemctl status ` + serviceName + ` --no-pager -l 2>/dev/null || true
exit 1`
}

func (service *Service) prepareBlueclawUpdateArtifact(upload *BlueclawUpdateUpload, archivePath string) (string, *blueclawUpdateArtifactMetadata, error) {
	artifactPath, errorValue := service.extractBlueclawUpdateArchive(archivePath)
	if errorValue != nil {
		return "", nil, errorValue
	}
	manifest, errorValue := blueclawruntime.ValidatePayloadArtifactDirectory(artifactPath)
	if errorValue != nil {
		return "", nil, errorValue
	}
	metadata, errorValue := service.persistBlueclawUpdateArtifact(upload, archivePath, artifactPath, manifest)
	if errorValue != nil {
		return "", nil, errorValue
	}
	return artifactPath, metadata, nil
}

func (service *Service) persistBlueclawUpdateArtifact(upload *BlueclawUpdateUpload, archivePath string, artifactPath string, manifest blueclawruntime.PayloadArtifactManifest) (*blueclawUpdateArtifactMetadata, error) {
	version := firstNonEmpty(upload.Version, manifest.BlueclawRevision)
	artifactDirectoryPath := filepath.Join(service.Configuration.StateDirectory, "blueclaw-updates", "artifacts", safeBlueclawUpdateVersion(version))
	if errorValue := os.MkdirAll(artifactDirectoryPath, 0o700); errorValue != nil {
		return nil, errorValue
	}
	storedArchivePath := filepath.Join(artifactDirectoryPath, "payload.tar.gz")
	if errorValue := copyFile(archivePath, storedArchivePath, 0o600); errorValue != nil {
		return nil, errorValue
	}
	manifestDocument, errorValue := os.ReadFile(filepath.Join(artifactPath, "manifest.json"))
	if errorValue != nil {
		return nil, errorValue
	}
	metadata := &blueclawUpdateArtifactMetadata{
		Component:        "blueclaw",
		Version:          version,
		BlueclawRevision: manifest.BlueclawRevision,
		SHA256:           fileSHA256(storedArchivePath),
		ManifestSHA256:   bytesSHA256(manifestDocument),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		Manifest:         manifest,
		ArchivePath:      storedArchivePath,
	}
	if errorValue := writeBlueclawUpdateMetadata(filepath.Join(artifactDirectoryPath, "metadata.json"), metadata); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := writeBlueclawUpdateMetadata(service.latestBlueclawUpdateMetadataPath(), metadata); errorValue != nil {
		return nil, errorValue
	}
	return metadata, nil
}

func (service *Service) extractBlueclawUpdateArchive(archivePath string) (string, error) {
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "blueclaw-updates", "staging", randomHex(12))
	if errorValue := os.MkdirAll(directoryPath, 0o700); errorValue != nil {
		return "", errorValue
	}
	if errorValue := extractTarGzip(archivePath, directoryPath); errorValue != nil {
		return "", errorValue
	}
	return directoryPath, nil
}

func extractTarGzip(archivePath string, targetDirectoryPath string) error {
	archiveFile, errorValue := os.Open(archivePath)
	if errorValue != nil {
		return errorValue
	}
	defer archiveFile.Close()
	gzipReader, errorValue := gzip.NewReader(archiveFile)
	if errorValue != nil {
		return errorValue
	}
	defer gzipReader.Close()
	return extractTar(gzipReader, targetDirectoryPath)
}

func extractTar(reader io.Reader, targetDirectoryPath string) error {
	tarReader := tar.NewReader(reader)
	for {
		header, errorValue := tarReader.Next()
		if errors.Is(errorValue, io.EOF) {
			return nil
		}
		if errorValue != nil {
			return errorValue
		}
		if errorValue := extractTarEntry(tarReader, header, targetDirectoryPath); errorValue != nil {
			return errorValue
		}
	}
}

func extractTarEntry(reader io.Reader, header *tar.Header, targetDirectoryPath string) error {
	targetPath, errorValue := safeTarTargetPath(targetDirectoryPath, header.Name)
	if errorValue != nil {
		return errorValue
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(targetPath, 0o700)
	case tar.TypeReg:
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o700); errorValue != nil {
			return errorValue
		}
		return writeReaderToFile(targetPath, reader, os.FileMode(header.Mode)&0o777)
	default:
		return nil
	}
}

func safeTarTargetPath(directoryPath string, name string) (string, error) {
	cleanName := filepath.Clean(strings.TrimPrefix(name, "/"))
	if cleanName == "." || strings.HasPrefix(cleanName, "..") {
		return "", errors.New("archive contains unsafe path")
	}
	targetPath := filepath.Join(directoryPath, cleanName)
	if !strings.HasPrefix(targetPath, filepath.Clean(directoryPath)+string(os.PathSeparator)) {
		return "", errors.New("archive contains path outside target")
	}
	return targetPath, nil
}

func writeReaderToFile(path string, reader io.Reader, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o600
	}
	file, errorValue := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(file, reader)
	closeErrorValue := file.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}

func writeLimitedRequestBody(path string, reader io.Reader, limit int64) error {
	file, errorValue := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return errorValue
	}
	copiedBytes, copyErrorValue := io.Copy(file, io.LimitReader(reader, limit+1))
	closeErrorValue := file.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	if copiedBytes > limit {
		return errors.New("request body exceeds limit")
	}
	return closeErrorValue
}

func (service *Service) findBlueclawUpdateUpload(uploadID string) (*BlueclawUpdateUpload, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload, isFound := service.blueclawUpdateUploads[uploadID]
	return upload, isFound
}

func (service *Service) markBlueclawUpdateUploadChunk(uploadID string, chunkIndex int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload := service.blueclawUpdateUploads[uploadID]
	if upload != nil {
		upload.ReceivedChunks[chunkIndex] = true
	}
}

func blueclawUpdateUploadTokenMatches(request *http.Request, upload *BlueclawUpdateUpload) bool {
	token := strings.TrimSpace(request.Header.Get("X-InternKim-Upload-Token"))
	if token == "" {
		token = strings.TrimPrefix(strings.TrimSpace(request.Header.Get("Authorization")), "Bearer ")
	}
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(upload.Token)) == 1
}

func parseBlueclawUpdateUploadChunkPath(path string) (string, int, bool) {
	trimmedPath := strings.TrimPrefix(path, "/updates/blueclaw/uploads/")
	parts := strings.Split(trimmedPath, "/chunks/")
	if len(parts) != 2 {
		return "", 0, false
	}
	chunkIndex, errorValue := strconv.Atoi(parts[1])
	return parts[0], chunkIndex, errorValue == nil && parts[0] != "" && chunkIndex >= 0
}

func validateBlueclawUpdateArchiveHash(path string, expectedSHA256 string) error {
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedSHA256 == "" {
		return nil
	}
	actualSHA256 := fileSHA256(path)
	if !strings.EqualFold(actualSHA256, expectedSHA256) {
		return fmt.Errorf("blueclaw update archive checksum mismatch")
	}
	return nil
}

func (service *Service) latestBlueclawUpdateMetadataPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "blueclaw-updates", "latest.json")
}

func (service *Service) readLatestBlueclawUpdateMetadata() *blueclawUpdateArtifactMetadata {
	return readBlueclawUpdateMetadata(service.latestBlueclawUpdateMetadataPath())
}

func (service *Service) readCurrentBlueclawUpdateMetadata() *blueclawUpdateArtifactMetadata {
	document, errorValue := os.ReadFile(blueclawruntime.BlueclawPayloadManifestPath)
	if errorValue != nil {
		return nil
	}
	manifest, errorValue := blueclawruntime.ParsePayloadArtifactManifest(document)
	if errorValue != nil {
		return nil
	}
	return &blueclawUpdateArtifactMetadata{
		Component:        "blueclaw",
		Version:          manifest.BlueclawRevision,
		BlueclawRevision: manifest.BlueclawRevision,
		ManifestSHA256:   bytesSHA256(document),
		CreatedAt:        "",
		Manifest:         manifest,
	}
}

func readBlueclawUpdateMetadata(path string) *blueclawUpdateArtifactMetadata {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return nil
	}
	var metadata blueclawUpdateArtifactMetadata
	if errorValue := json.Unmarshal(document, &metadata); errorValue != nil {
		return nil
	}
	return &metadata
}

func writeBlueclawUpdateMetadata(path string, metadata *blueclawUpdateArtifactMetadata) error {
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(metadata, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func publicBlueclawUpdateMetadata(metadata *blueclawUpdateArtifactMetadata) *blueclawUpdateArtifactMetadata {
	if metadata == nil {
		return nil
	}
	publicMetadata := *metadata
	publicMetadata.ArchivePath = ""
	return &publicMetadata
}

func blueclawUpdateState(current *blueclawUpdateArtifactMetadata, latest *blueclawUpdateArtifactMetadata) string {
	if latest == nil {
		return "idle"
	}
	if current != nil && current.BlueclawRevision == latest.BlueclawRevision {
		return "already_current"
	}
	return "idle"
}

func (service *Service) activeBlueclawUpdateJob() *Job {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, job := range service.jobs {
		if job.Type == "blueclaw-update" && (job.Status == "pending" || job.Status == "running") {
			return job
		}
	}
	return nil
}

func fileSHA256(path string) string {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	_, _ = io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil))
}

func bytesSHA256(document []byte) string {
	hash := sha256.Sum256(document)
	return hex.EncodeToString(hash[:])
}

func copyFile(sourcePath string, targetPath string, mode os.FileMode) error {
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o700); errorValue != nil {
		return errorValue
	}
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(targetFile, sourceFile)
	closeErrorValue := targetFile.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}

func safeBlueclawUpdateVersion(version string) string {
	normalizedVersion := strings.ToLower(strings.TrimSpace(version))
	normalizedVersion = strings.Map(func(character rune) rune {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' {
			return character
		}
		return '-'
	}, normalizedVersion)
	normalizedVersion = strings.Trim(normalizedVersion, "-_.")
	if normalizedVersion == "" {
		return randomHex(8)
	}
	return normalizedVersion
}

func quoteBlueclawUpdateShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
