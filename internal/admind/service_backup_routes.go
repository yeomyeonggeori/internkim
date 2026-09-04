package admind

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"strconv"
	"strings"
	"time"
)

type Job struct {
	JobID        string            `json:"jobID"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	Phase        string            `json:"phase"`
	Error        string            `json:"error,omitempty"`
	CreatedAt    time.Time         `json:"createdAt"`
	UpdatedAt    time.Time         `json:"updatedAt"`
	DownloadURL  string            `json:"downloadURL,omitempty"`
	Manifest     *BackupManifest   `json:"manifest,omitempty"`
	Logs         []string          `json:"logs"`
	Result       map[string]string `json:"result,omitempty"`
	artifactPath string
}

type RestoreUpload struct {
	UploadID       string       `json:"uploadID"`
	Filename       string       `json:"filename"`
	Size           int64        `json:"size"`
	CreatedAt      time.Time    `json:"createdAt"`
	DirectoryPath  string       `json:"-"`
	ReceivedChunks map[int]bool `json:"-"`
}

type backupRequest struct {
	Passphrase string `json:"passphrase"`
}

type restoreRequest struct {
	Passphrase string `json:"passphrase"`
	Confirm    string `json:"confirm"`
}

type restoreUploadRequest struct {
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
}

type restoreUploadResponse struct {
	UploadID  string `json:"uploadID"`
	ChunkSize int64  `json:"chunkSize"`
}

type restoreUploadCompleteRequest struct {
	Passphrase string `json:"passphrase"`
	Confirm    string `json:"confirm"`
	Chunks     int    `json:"chunks"`
}

func (service *Service) createRestoreUpload(responseWriter http.ResponseWriter, request *http.Request) {
	var payload restoreUploadRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	uploadID := randomHex(16)
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "uploads", uploadID)
	upload := &RestoreUpload{
		UploadID:       uploadID,
		Filename:       filepath.Base(payload.Filename),
		Size:           payload.Size,
		CreatedAt:      time.Now().UTC(),
		DirectoryPath:  directoryPath,
		ReceivedChunks: map[int]bool{},
	}
	service.mutex.Lock()
	if errorValue := os.MkdirAll(filepath.Join(directoryPath, "chunks"), 0o700); errorValue != nil {
		service.mutex.Unlock()
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.uploads[uploadID] = upload
	service.mutex.Unlock()
	service.writeJSON(responseWriter, restoreUploadResponse{UploadID: uploadID, ChunkSize: 4 << 20})
}

func (service *Service) writeRestoreUploadChunk(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID, chunkIndex, isValid := parseRestoreUploadChunkPath(path)
	if !isValid {
		http.NotFound(responseWriter, request)
		return
	}
	upload, isFound := service.findUpload(uploadID)
	if !isFound {
		http.NotFound(responseWriter, request)
		return
	}
	chunkPath := filepath.Join(upload.DirectoryPath, "chunks", strconv.Itoa(chunkIndex))
	chunkFile, errorValue := os.OpenFile(chunkPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	_, copyErrorValue := io.Copy(chunkFile, io.LimitReader(request.Body, 16<<20))
	closeErrorValue := chunkFile.Close()
	if copyErrorValue != nil {
		http.Error(responseWriter, copyErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	if closeErrorValue != nil {
		http.Error(responseWriter, closeErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.markUploadChunk(uploadID, chunkIndex)
	service.writeJSON(responseWriter, map[string]string{"status": "ok"})
}

func (service *Service) completeRestoreUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID := strings.TrimSuffix(strings.TrimPrefix(path, "/restore/uploads/"), "/complete")
	upload, isFound := service.findUpload(uploadID)
	if !isFound {
		http.NotFound(responseWriter, request)
		return
	}
	var payload restoreUploadCompleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Passphrase) == "" {
		http.Error(responseWriter, "passphrase is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Confirm) != "RESTORE" {
		http.Error(responseWriter, "confirm must be RESTORE", http.StatusBadRequest)
		return
	}
	if payload.Chunks <= 0 {
		http.Error(responseWriter, "chunks is required", http.StatusBadRequest)
		return
	}
	bundlePath := filepath.Join(upload.DirectoryPath, firstNonEmpty(upload.Filename, "restore.ikbak"))
	if errorValue := service.assembleRestoreUpload(upload, payload.Chunks, bundlePath); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	job := service.newJob("restore")
	service.writeJSON(responseWriter, job)
	go service.runRestoreJob(context.Background(), job.JobID, bundlePath, payload.Passphrase)
}

func (service *Service) createBackup(responseWriter http.ResponseWriter, request *http.Request) {
	var payload backupRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Passphrase) == "" {
		http.Error(responseWriter, "passphrase is required", http.StatusBadRequest)
		return
	}
	job := service.newJob("backup")
	service.writeJSON(responseWriter, job)
	go service.runBackupJob(context.Background(), job.JobID, payload.Passphrase)
}

func (service *Service) createRestore(responseWriter http.ResponseWriter, request *http.Request) {
	if errorValue := request.ParseMultipartForm(64 << 20); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	payload := restoreRequest{
		Passphrase: request.FormValue("passphrase"),
		Confirm:    request.FormValue("confirm"),
	}
	if strings.TrimSpace(payload.Passphrase) == "" {
		http.Error(responseWriter, "passphrase is required", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.Confirm) != "RESTORE" {
		http.Error(responseWriter, "confirm must be RESTORE", http.StatusBadRequest)
		return
	}
	file, header, errorValue := request.FormFile("bundle")
	if errorValue != nil {
		http.Error(responseWriter, "bundle is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	job := service.newJob("restore")
	uploadPath := filepath.Join(service.jobDirectory(job.JobID), filepath.Base(header.Filename))
	if errorValue := os.MkdirAll(filepath.Dir(uploadPath), 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	uploadFile, errorValue := os.Create(uploadPath)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	_, copyErrorValue := io.Copy(uploadFile, file)
	closeErrorValue := uploadFile.Close()
	if copyErrorValue != nil {
		http.Error(responseWriter, copyErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	if closeErrorValue != nil {
		http.Error(responseWriter, closeErrorValue.Error(), http.StatusInternalServerError)
		return
	}

	service.writeJSON(responseWriter, job)
	go service.runRestoreJob(context.Background(), job.JobID, uploadPath, payload.Passphrase)
}

func (service *Service) runBackupJob(ctx context.Context, jobID string, passphrase string) {
	job := service.mustJob(jobID)
	service.updateJob(jobID, "running", "collecting", "")
	jobDirectory := service.jobDirectory(jobID)
	plainPath := filepath.Join(jobDirectory, "internkim-backup.tar.gz")
	encryptedPath := filepath.Join(jobDirectory, "internkim-backup.ikbak")

	blueclawManifest, completeBlueclawBackup := service.prepareBlueclawBackup(ctx)
	defer completeBlueclawBackup()
	manifest, errorValue := service.createPlainBackup(ctx, plainPath, blueclawManifest)
	if errorValue != nil {
		service.updateJob(jobID, "failed", "collecting", errorValue.Error())
		return
	}
	service.updateJobManifest(jobID, manifest)
	service.updateJob(jobID, "running", "encrypting", "")
	if errorValue := encryptFile(plainPath, encryptedPath, passphrase); errorValue != nil {
		service.updateJob(jobID, "failed", "encrypting", errorValue.Error())
		return
	}
	_ = os.Remove(plainPath)

	job.artifactPath = encryptedPath
	job.DownloadURL = "/admin/api/backups/" + jobID + "/download"
	service.updateJob(jobID, "completed", "ready", "")
}

func (service *Service) createPlainBackup(ctx context.Context, plainPath string, blueclawManifest map[string]any) (*BackupManifest, error) {
	if errorValue := os.MkdirAll(filepath.Dir(plainPath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	manifest := service.newBackupManifest(blueclawManifest)

	plainFile, errorValue := os.Create(plainPath)
	if errorValue != nil {
		return nil, errorValue
	}
	defer plainFile.Close()
	gzipWriter := gzip.NewWriter(plainFile)
	defer gzipWriter.Close()
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	for _, includedPath := range backupIncludedPaths() {
		if errorValue := addPathToTar(tarWriter, includedPath, manifest); errorValue != nil && !os.IsNotExist(errorValue) {
			return nil, errorValue
		}
	}
	if errorValue := service.addDatabaseDumpsToTar(ctx, tarWriter, manifest); errorValue != nil {
		return nil, errorValue
	}
	if errorValue := addManifestToTar(tarWriter, manifest); errorValue != nil {
		return nil, errorValue
	}
	return manifest, nil
}

func (service *Service) newBackupManifest(blueclawManifest map[string]any) *BackupManifest {
	return &BackupManifest{
		FormatVersion: 1,
		FleetID:       readTrimmedFile(service.Configuration.FleetIDPath),
		CreatedAt:     time.Now().UTC(),
		Components: []string{
			"internkim",
			"blueclaw",
			"mattermost",
			"cloudflared",
		},
		Checksums: map[string]string{},
		InternKim: map[string]string{
			"backupFormat": "internkim-admin-v1",
		},
		Blueclaw: blueclawManifest,
	}
}

func (service *Service) addDatabaseDumpsToTar(ctx context.Context, tarWriter *tar.Writer, manifest *BackupManifest) error {
	dumpPath, errorValue := service.dumpMattermostDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	if dumpPath != "" {
		defer os.Remove(dumpPath)
		manifest.MattermostDump = true
		if errorValue := addNamedFileToTar(tarWriter, dumpPath, "mattermost-db.sql", manifest); errorValue != nil {
			return errorValue
		}
	}
	blueclawDumpPath, errorValue := service.dumpBlueclawDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	if blueclawDumpPath != "" {
		defer os.Remove(blueclawDumpPath)
		manifest.BlueclawDump = true
		if errorValue := addNamedFileToTar(tarWriter, blueclawDumpPath, "blueclaw-db.sql", manifest); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) runRestoreJob(ctx context.Context, jobID string, encryptedPath string, passphrase string) {
	jobDirectory := service.jobDirectory(jobID)
	plainPath := filepath.Join(jobDirectory, "restore.tar.gz")
	extractDirectory := filepath.Join(jobDirectory, "extract")

	service.updateJob(jobID, "running", "decrypting", "")
	if errorValue := decryptFile(encryptedPath, plainPath, passphrase); errorValue != nil {
		service.updateJob(jobID, "failed", "decrypting", errorValue.Error())
		return
	}
	service.updateJob(jobID, "running", "extracting", "")
	manifest, errorValue := extractBundle(plainPath, extractDirectory)
	if errorValue != nil {
		service.updateJob(jobID, "failed", "extracting", errorValue.Error())
		return
	}
	service.updateJobManifest(jobID, manifest)

	service.updateJob(jobID, "running", "applying", "")
	if errorValue := service.applyRestore(ctx, extractDirectory); errorValue != nil {
		service.updateJob(jobID, "failed", "applying", errorValue.Error())
		return
	}
	service.updateJob(jobID, "completed", "restarting", "")
}

func (service *Service) applyRestore(ctx context.Context, extractDirectory string) error {
	commands := [][]string{
		{"systemctl", "stop", "blueclaw", "internkim-capabilityd", "mattermost", "internkim-users-sync.timer", "internkim-users-sync.service"},
	}
	for _, arguments := range commands {
		_, _ = service.runCommand(ctx, arguments[0], arguments[1:]...)
	}

	for _, relativePath := range []string{"root/.internkim", "root/.blueclaw", "opt/mattermost/config", "opt/mattermost/data", "etc/cloudflared"} {
		sourcePath := filepath.Join(extractDirectory, relativePath)
		if _, errorValue := os.Stat(sourcePath); errorValue == nil {
			targetPath := "/" + relativePath
			if errorValue := copyDirectory(sourcePath, targetPath); errorValue != nil {
				return errorValue
			}
		}
	}
	for _, relativePath := range []string{"root/.internkim/admin-email", "root/.internkim/claimed-admin-email"} {
		sourcePath := filepath.Join(extractDirectory, relativePath)
		if _, errorValue := os.Stat(sourcePath); errorValue == nil {
			if errorValue := copyRegularFile(sourcePath, "/"+relativePath); errorValue != nil {
				return errorValue
			}
		}
	}

	databaseDumpPath := filepath.Join(extractDirectory, "mattermost-db.sql")
	if _, errorValue := os.Stat(databaseDumpPath); errorValue == nil {
		_, _ = service.runCommand(ctx, "systemctl", "start", "postgresql")
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "dropdb --if-exists mattermost && createdb mattermost"); errorValue != nil {
			return errorValue
		}
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql mattermost < "+shellQuote(databaseDumpPath)); errorValue != nil {
			return errorValue
		}
	}
	blueclawDatabaseDumpPath := filepath.Join(extractDirectory, "blueclaw-db.sql")
	if _, errorValue := os.Stat(blueclawDatabaseDumpPath); errorValue == nil {
		_, _ = service.runCommand(ctx, "systemctl", "start", "postgresql")
		_, _ = service.runCommand(ctx, "su", "-", "postgres", "-c", "psql -tAc "+shellQuote("SELECT 1 FROM pg_roles WHERE rolname='blueclaw'")+" | grep -q 1 || createuser blueclaw")
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "dropdb --if-exists blueclaw && createdb -O blueclaw blueclaw"); errorValue != nil {
			return errorValue
		}
		if _, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql blueclaw < "+shellQuote(blueclawDatabaseDumpPath)); errorValue != nil {
			return errorValue
		}
	}

	_, _ = service.runCommand(ctx, "systemctl", "daemon-reload")
	_, _ = service.runCommand(ctx, "systemctl", "restart", "mattermost", "internkim-capabilityd", "blueclaw", "cloudflared")
	_, _ = service.runCommand(ctx, "systemctl", "enable", "--now", "internkim-users-sync.timer")
	return nil
}

func (service *Service) writeJob(responseWriter http.ResponseWriter, jobID string) {
	job, isFound := service.findJob(jobID)
	if !isFound {
		http.NotFound(responseWriter, nil)
		return
	}
	service.writeJSON(responseWriter, job)
}

func (service *Service) downloadBackup(responseWriter http.ResponseWriter, request *http.Request, jobID string) {
	job, isFound := service.findJob(jobID)
	if !isFound || job.Status != "completed" || strings.TrimSpace(job.artifactPath) == "" {
		http.NotFound(responseWriter, nil)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/octet-stream")
	responseWriter.Header().Set("Content-Disposition", `attachment; filename="internkim-backup-`+jobID+`.ikbak"`)
	http.ServeFile(responseWriter, request, job.artifactPath)
}

func (service *Service) newJob(jobType string) *Job {
	jobID := randomHex(16)
	now := time.Now().UTC()
	job := &Job{
		JobID:     jobID,
		Type:      jobType,
		Status:    "queued",
		Phase:     "queued",
		CreatedAt: now,
		UpdatedAt: now,
		Logs:      []string{},
		Result:    map[string]string{},
	}
	service.mutex.Lock()
	service.jobs[jobID] = job
	service.mutex.Unlock()
	return job
}

func (service *Service) mustJob(jobID string) *Job {
	job, _ := service.findJob(jobID)
	return job
}

func (service *Service) findJob(jobID string) (*Job, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job, isFound := service.jobs[jobID]
	return job, isFound
}

func (service *Service) findUpload(uploadID string) (*RestoreUpload, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload, isFound := service.uploads[uploadID]
	return upload, isFound
}

func (service *Service) markUploadChunk(uploadID string, chunkIndex int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload := service.uploads[uploadID]
	if upload == nil {
		return
	}
	upload.ReceivedChunks[chunkIndex] = true
}

func (service *Service) assembleRestoreUpload(upload *RestoreUpload, chunkCount int, bundlePath string) error {
	return assembleChunkDirectory(upload.DirectoryPath, chunkCount, bundlePath, "restore upload")
}

func assembleChunkDirectory(directoryPath string, chunkCount int, targetPath string, label string) error {
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return errorValue
	}
	defer targetFile.Close()
	for chunkIndex := 0; chunkIndex < chunkCount; chunkIndex++ {
		chunkPath := filepath.Join(directoryPath, "chunks", strconv.Itoa(chunkIndex))
		chunkFile, errorValue := os.Open(chunkPath)
		if errorValue != nil {
			return errors.New(label + " is missing chunk " + strconv.Itoa(chunkIndex))
		}
		_, copyErrorValue := io.Copy(targetFile, chunkFile)
		closeErrorValue := chunkFile.Close()
		if copyErrorValue != nil {
			return copyErrorValue
		}
		if closeErrorValue != nil {
			return closeErrorValue
		}
	}
	return nil
}

func (service *Service) updateJob(jobID string, status string, phase string, errorMessage string) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.jobs[jobID]
	if job == nil {
		return
	}
	job.Status = status
	job.Phase = phase
	job.Error = errorMessage
	job.UpdatedAt = time.Now().UTC()
	job.Logs = append(job.Logs, phase)
}

func (service *Service) updateJobManifest(jobID string, manifest *BackupManifest) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.jobs[jobID]
	if job == nil {
		return
	}
	job.Manifest = manifest
	job.UpdatedAt = time.Now().UTC()
}

func (service *Service) jobDirectory(jobID string) string {
	return filepath.Join(service.Configuration.StateDirectory, "jobs", jobID)
}

func (service *Service) dumpMattermostDatabase(ctx context.Context) (string, error) {
	dumpPath := filepath.Join(os.TempDir(), "internkim-mattermost-"+randomHex(8)+".sql")
	command := "pg_dump mattermost > " + shellQuote(dumpPath)
	_, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", command)
	if errorValue != nil {
		return "", errorValue
	}
	return dumpPath, nil
}

func (service *Service) dumpBlueclawDatabase(ctx context.Context) (string, error) {
	output, errorValue := service.runCommand(ctx, "su", "-", "postgres", "-c", "psql -tAc "+shellQuote("SELECT 1 FROM pg_database WHERE datname='blueclaw'"))
	if errorValue != nil {
		return "", nil
	}
	exists := strings.TrimSpace(string(output))
	if exists != "1" {
		return "", nil
	}
	dumpPath := filepath.Join(os.TempDir(), "internkim-blueclaw-"+randomHex(8)+".sql")
	command := "pg_dump blueclaw > " + shellQuote(dumpPath)
	_, errorValue = service.runCommand(ctx, "su", "-", "postgres", "-c", command)
	if errorValue != nil {
		return "", errorValue
	}
	return dumpPath, nil
}

func (service *Service) prepareBlueclawBackup(ctx context.Context) (map[string]any, func()) {
	manifest := service.fetchBlueclawManifest(ctx)
	if manifest == nil {
		return nil, func() {}
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.BlueclawBaseURL+"/admin/api/backup/prepare", strings.NewReader(`{"holder":"internkim-admind"}`))
	if errorValue != nil {
		return manifest, func() {}
	}
	request.Header.Set("Content-Type", "application/json")
	client := service.httpClient()
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return manifest, func() {}
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return manifest, func() {}
	}
	return manifest, func() {
		completeRequest, errorValue := http.NewRequestWithContext(context.Background(), http.MethodPost, service.Configuration.BlueclawBaseURL+"/admin/api/backup/complete", nil)
		if errorValue != nil {
			return
		}
		completeResponse, errorValue := client.Do(completeRequest)
		if errorValue == nil {
			_ = completeResponse.Body.Close()
		}
	}
}

func (service *Service) fetchBlueclawManifest(ctx context.Context) map[string]any {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, service.Configuration.BlueclawBaseURL+"/admin/api/backup/manifest", nil)
	if errorValue != nil {
		return nil
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil
	}
	var manifest map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&manifest); errorValue != nil {
		return nil
	}
	return manifest
}

func parseRestoreUploadChunkPath(path string) (string, int, bool) {
	trimmedPath := strings.TrimPrefix(path, "/restore/uploads/")
	parts := strings.Split(trimmedPath, "/chunks/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", 0, false
	}
	chunkIndex, errorValue := strconv.Atoi(parts[1])
	if errorValue != nil || chunkIndex < 0 {
		return "", 0, false
	}
	return parts[0], chunkIndex, true
}
