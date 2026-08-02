package admind

import (
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
	"unicode"
)

const (
	companionFileChunkSize         = 4 << 20
	defaultCompanionFileMaxBytes   = 64 << 20
	defaultCompanionFileTTLSeconds = 24 * 60 * 60
	minCompanionFileTTLSeconds     = 5 * 60
	maxCompanionFileTTLSeconds     = 7 * 24 * 60 * 60
)

type CompanionFileUpload struct {
	UploadID       string
	JobID          string
	CompanionID    string
	Filename       string
	ContentType    string
	SizeBytes      int64
	ExpiresAt      time.Time
	DirectoryPath  string
	ReceivedChunks map[int]bool
}

type companionFileUploadCreateRequest struct {
	JobID       string `json:"jobID"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType"`
	TTLSeconds  int    `json:"ttlSeconds"`
}

type companionFileUploadCreateResponse struct {
	UploadID  string `json:"uploadID"`
	ChunkSize int64  `json:"chunkSize"`
}

type companionFileUploadCompleteRequest struct {
	Chunks int `json:"chunks"`
}

type companionFileUploadCompleteResponse struct {
	FileID      string    `json:"fileID"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"sizeBytes"`
	ContentType string    `json:"contentType"`
	DevicePath  string    `json:"devicePath"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type companionFileMetadata struct {
	FileID      string    `json:"fileID"`
	Filename    string    `json:"filename"`
	DevicePath  string    `json:"devicePath"`
	ExpiresAt   time.Time `json:"expiresAt"`
	CompanionID string    `json:"companionID"`
	JobID       string    `json:"jobID"`
}

func (service *Service) createCompanionFileUpload(responseWriter http.ResponseWriter, request *http.Request) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	var payload companionFileUploadCreateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.validateCompanionFileUpload(companion.CompanionID, payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	uploadID := randomHex(16)
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "companion-file-uploads", uploadID)
	if errorValue := os.MkdirAll(filepath.Join(directoryPath, "chunks"), 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	upload := &CompanionFileUpload{
		UploadID:       uploadID,
		JobID:          strings.TrimSpace(payload.JobID),
		CompanionID:    companion.CompanionID,
		Filename:       safeCompanionFilename(payload.Filename),
		ContentType:    firstNonEmpty(payload.ContentType, "application/octet-stream"),
		SizeBytes:      payload.SizeBytes,
		ExpiresAt:      time.Now().UTC().Add(time.Duration(clampCompanionFileTTL(payload.TTLSeconds)) * time.Second),
		DirectoryPath:  directoryPath,
		ReceivedChunks: map[int]bool{},
	}
	service.mutex.Lock()
	service.companionFileUploads[uploadID] = upload
	service.mutex.Unlock()
	service.writeJSON(responseWriter, companionFileUploadCreateResponse{UploadID: uploadID, ChunkSize: companionFileChunkSize})
}

func (service *Service) writeCompanionFileUploadChunk(responseWriter http.ResponseWriter, request *http.Request, path string) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	uploadID, chunkIndex, isValid := parseCompanionFileUploadChunkPath(path)
	if !isValid {
		http.NotFound(responseWriter, request)
		return
	}
	upload, isFound := service.findCompanionFileUpload(uploadID)
	if !isFound || upload.CompanionID != companion.CompanionID {
		http.NotFound(responseWriter, request)
		return
	}
	chunkPath := filepath.Join(upload.DirectoryPath, "chunks", strconv.Itoa(chunkIndex))
	chunkFile, errorValue := os.OpenFile(chunkPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	copiedBytes, copyErrorValue := io.Copy(chunkFile, io.LimitReader(request.Body, companionFileChunkSize+1))
	closeErrorValue := chunkFile.Close()
	if copyErrorValue != nil {
		http.Error(responseWriter, copyErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	if copiedBytes > companionFileChunkSize {
		http.Error(responseWriter, "file upload chunk is too large", http.StatusBadRequest)
		return
	}
	if closeErrorValue != nil {
		http.Error(responseWriter, closeErrorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.markCompanionFileUploadChunk(uploadID, chunkIndex)
	service.writeJSON(responseWriter, map[string]string{"status": "ok"})
}

func (service *Service) completeCompanionFileUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	companion := service.authorizedCompanion(request)
	if companion == nil {
		http.Error(responseWriter, "companion auth required", http.StatusForbidden)
		return
	}
	uploadID := strings.TrimSuffix(strings.TrimPrefix(path, "/files/uploads/"), "/complete")
	upload, isFound := service.findCompanionFileUpload(uploadID)
	if !isFound || upload.CompanionID != companion.CompanionID {
		http.NotFound(responseWriter, request)
		return
	}
	var payload companionFileUploadCompleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	result, errorValue := service.finishCompanionFileUpload(upload, payload.Chunks)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, result)
}

func (service *Service) validateCompanionFileUpload(companionID string, payload companionFileUploadCreateRequest) error {
	if strings.TrimSpace(payload.JobID) == "" {
		return errors.New("jobID is required")
	}
	if payload.SizeBytes < 0 || payload.SizeBytes > defaultCompanionFileMaxBytes {
		return errors.New("file size exceeds companion upload limit")
	}
	if safeCompanionFilename(payload.Filename) == "" {
		return errors.New("filename is required")
	}
	service.mutex.Lock()
	defer service.mutex.Unlock()
	job := service.companionJobs[strings.TrimSpace(payload.JobID)]
	if job == nil || job.CompanionID != companionID || job.Status != "running" || job.ToolName != "file_pick" {
		return errors.New("companion file upload is not allowed for this job")
	}
	return nil
}

func (service *Service) finishCompanionFileUpload(upload *CompanionFileUpload, chunkCount int) (companionFileUploadCompleteResponse, error) {
	if chunkCount < 0 {
		return companionFileUploadCompleteResponse{}, errors.New("chunks is invalid")
	}
	if errorValue := os.MkdirAll(service.Configuration.CompanionFileDirectory, 0o700); errorValue != nil {
		return companionFileUploadCompleteResponse{}, errorValue
	}
	targetPath := filepath.Join(service.Configuration.CompanionFileDirectory, upload.Filename)
	temporaryPath := targetPath + ".uploading"
	if errorValue := service.assembleCompanionFileUpload(upload, chunkCount, temporaryPath); errorValue != nil {
		return companionFileUploadCompleteResponse{}, errorValue
	}
	if errorValue := os.Rename(temporaryPath, targetPath); errorValue != nil {
		_ = os.Remove(targetPath)
		if renameError := os.Rename(temporaryPath, targetPath); renameError != nil {
			return companionFileUploadCompleteResponse{}, errorValue
		}
	}
	information, errorValue := os.Stat(targetPath)
	if errorValue != nil {
		return companionFileUploadCompleteResponse{}, errorValue
	}
	if information.Size() != upload.SizeBytes {
		return companionFileUploadCompleteResponse{}, errors.New("assembled file size does not match upload metadata")
	}
	_ = os.Chmod(targetPath, 0o600)
	metadata := companionFileMetadata{
		FileID:      upload.UploadID,
		Filename:    upload.Filename,
		DevicePath:  targetPath,
		ExpiresAt:   upload.ExpiresAt,
		CompanionID: upload.CompanionID,
		JobID:       upload.JobID,
	}
	if errorValue := writeCompanionFileMetadata(metadata); errorValue != nil {
		return companionFileUploadCompleteResponse{}, errorValue
	}
	return companionFileUploadCompleteResponse{
		FileID:      upload.UploadID,
		Filename:    upload.Filename,
		SizeBytes:   upload.SizeBytes,
		ContentType: upload.ContentType,
		DevicePath:  targetPath,
		ExpiresAt:   upload.ExpiresAt,
	}, nil
}

func (service *Service) assembleCompanionFileUpload(upload *CompanionFileUpload, chunkCount int, targetPath string) error {
	file, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	for chunkIndex := 0; chunkIndex < chunkCount; chunkIndex++ {
		if !upload.ReceivedChunks[chunkIndex] {
			return errors.New("file upload is missing chunk " + strconv.Itoa(chunkIndex))
		}
		chunkPath := filepath.Join(upload.DirectoryPath, "chunks", strconv.Itoa(chunkIndex))
		chunkFile, errorValue := os.Open(chunkPath)
		if errorValue != nil {
			return errorValue
		}
		_, copyErrorValue := io.Copy(file, chunkFile)
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

func (service *Service) findCompanionFileUpload(uploadID string) (*CompanionFileUpload, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload, isFound := service.companionFileUploads[uploadID]
	return upload, isFound
}

func (service *Service) markCompanionFileUploadChunk(uploadID string, chunkIndex int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload := service.companionFileUploads[uploadID]
	if upload != nil {
		upload.ReceivedChunks[chunkIndex] = true
	}
}

func (service *Service) startCompanionFileCleanup(ctx context.Context) {
	_ = os.MkdirAll(service.Configuration.CompanionFileDirectory, 0o700)
	_ = service.cleanupExpiredCompanionFiles(time.Now().UTC())
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				_ = service.cleanupExpiredCompanionFiles(now.UTC())
			}
		}
	}()
}

func (service *Service) cleanupExpiredCompanionFiles(now time.Time) error {
	entries, errorValue := os.ReadDir(service.Configuration.CompanionFileDirectory)
	if errorValue != nil {
		if errors.Is(errorValue, os.ErrNotExist) {
			return nil
		}
		return errorValue
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".internkim-meta.json") {
			continue
		}
		metadataPath := filepath.Join(service.Configuration.CompanionFileDirectory, entry.Name())
		metadata, errorValue := readCompanionFileMetadata(metadataPath)
		if errorValue != nil || now.Before(metadata.ExpiresAt) {
			continue
		}
		if isCompanionTempPath(service.Configuration.CompanionFileDirectory, metadata.DevicePath) {
			_ = os.Remove(metadata.DevicePath)
		}
		_ = os.Remove(metadataPath)
	}
	return nil
}

func writeCompanionFileMetadata(metadata companionFileMetadata) error {
	document, errorValue := json.MarshalIndent(metadata, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(metadata.DevicePath+".internkim-meta.json", document, 0o600)
}

func readCompanionFileMetadata(path string) (companionFileMetadata, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return companionFileMetadata{}, errorValue
	}
	var metadata companionFileMetadata
	if errorValue := json.Unmarshal(document, &metadata); errorValue != nil {
		return companionFileMetadata{}, errorValue
	}
	return metadata, nil
}

func parseCompanionFileUploadChunkPath(path string) (string, int, bool) {
	trimmedPath := strings.TrimPrefix(path, "/files/uploads/")
	parts := strings.Split(trimmedPath, "/chunks/")
	if len(parts) != 2 {
		return "", 0, false
	}
	chunkIndex, errorValue := strconv.Atoi(parts[1])
	return parts[0], chunkIndex, errorValue == nil && parts[0] != "" && chunkIndex >= 0
}

func clampCompanionFileTTL(ttlSeconds int) int {
	if ttlSeconds <= 0 {
		return defaultCompanionFileTTLSeconds
	}
	if ttlSeconds < minCompanionFileTTLSeconds {
		return minCompanionFileTTLSeconds
	}
	if ttlSeconds > maxCompanionFileTTLSeconds {
		return maxCompanionFileTTLSeconds
	}
	return ttlSeconds
}

func safeCompanionFilename(filename string) string {
	baseName := filepath.Base(strings.TrimSpace(filename))
	if baseName == "." || baseName == ".." {
		return ""
	}
	builder := strings.Builder{}
	for _, value := range baseName {
		switch {
		case value == '/' || value == '\\' || unicode.IsControl(value):
			continue
		default:
			builder.WriteRune(value)
		}
	}
	return strings.TrimSpace(builder.String())
}

func isCompanionTempPath(directory string, path string) bool {
	relativePath, errorValue := filepath.Rel(directory, path)
	return errorValue == nil && !strings.HasPrefix(relativePath, "..") && !filepath.IsAbs(relativePath)
}
