package admind

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

const releaseUpdateUploadAction = "release-update-upload"
const releaseUpdateChunkSize = 4 << 20

type ReleaseUpdateUpload struct {
	UploadID       string
	Token          string
	ReleaseID      string
	Filename       string
	Size           int64
	SHA256         string
	CreatedAt      time.Time
	DirectoryPath  string
	ReceivedChunks map[int]bool
}

type releaseUpdateUploadCreateRequest struct {
	fleetSignedRequest
	ReleaseID string `json:"releaseID"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

type releaseUpdateUploadCreateResponse struct {
	UploadID       string `json:"uploadID"`
	UploadToken    string `json:"uploadToken"`
	ChunkSize      int    `json:"chunkSize"`
	ReceivedChunks []int  `json:"receivedChunks,omitempty"`
}

type releaseUpdateUploadCompleteRequest struct {
	Chunks int    `json:"chunks"`
	SHA256 string `json:"sha256"`
}

func (service *Service) handleReleaseUpdateUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	switch {
	case request.Method == http.MethodPost && path == "/updates/uploads":
		service.createReleaseUpdateUpload(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/updates/uploads/") && strings.Contains(path, "/chunks/"):
		service.writeReleaseUpdateUploadChunk(responseWriter, request, path)
	case request.Method == http.MethodPost && strings.HasPrefix(path, "/updates/uploads/") && strings.HasSuffix(path, "/complete"):
		service.completeReleaseUpdateUpload(responseWriter, request, path)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) createReleaseUpdateUpload(responseWriter http.ResponseWriter, request *http.Request) {
	var payload releaseUpdateUploadCreateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.validateFleetSignedRequest(payload.fleetSignedRequest, isAllowedReleaseUpdateUploadAction); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusForbidden)
		return
	}
	if errorValue := validateReleaseUpdateUploadRequest(payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	releaseID := strings.TrimSpace(payload.ReleaseID)
	sha256Value := strings.ToLower(strings.TrimSpace(payload.SHA256))
	if existing, receivedChunks, isFound := service.findResumableReleaseUpdateUpload(releaseID, sha256Value); isFound {
		service.writeJSON(responseWriter, releaseUpdateUploadCreateResponse{
			UploadID:       existing.UploadID,
			UploadToken:    existing.Token,
			ChunkSize:      releaseUpdateChunkSize,
			ReceivedChunks: receivedChunks,
		})
		return
	}
	uploadID := randomHex(16)
	uploadToken := randomHex(24)
	directoryPath := filepath.Join(service.Configuration.StateDirectory, "release-updates", "uploads", uploadID)
	if errorValue := os.MkdirAll(filepath.Join(directoryPath, "chunks"), 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.mutex.Lock()
	service.releaseUpdateUploads[uploadID] = &ReleaseUpdateUpload{
		UploadID:       uploadID,
		Token:          uploadToken,
		ReleaseID:      strings.TrimSpace(payload.ReleaseID),
		Filename:       filepath.Base(payload.Filename),
		Size:           payload.Size,
		SHA256:         strings.ToLower(strings.TrimSpace(payload.SHA256)),
		CreatedAt:      time.Now().UTC(),
		DirectoryPath:  directoryPath,
		ReceivedChunks: map[int]bool{},
	}
	service.mutex.Unlock()
	service.writeJSON(responseWriter, releaseUpdateUploadCreateResponse{UploadID: uploadID, UploadToken: uploadToken, ChunkSize: releaseUpdateChunkSize})
}

func isAllowedReleaseUpdateUploadAction(action string) bool {
	return action == releaseUpdateUploadAction
}

func validateReleaseUpdateUploadRequest(payload releaseUpdateUploadCreateRequest) error {
	if payload.Size <= 0 {
		return errors.New("release upload bundle size is required")
	}
	sha256Value := strings.TrimSpace(payload.SHA256)
	if len(sha256Value) != 64 {
		return errors.New("release upload bundle sha256 is invalid")
	}
	if _, errorValue := hex.DecodeString(sha256Value); errorValue != nil {
		return errors.New("release upload bundle sha256 is invalid")
	}
	return nil
}

func (service *Service) writeReleaseUpdateUploadChunk(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID, chunkIndex, isValid := parseReleaseUpdateUploadChunkPath(path)
	if !isValid {
		http.NotFound(responseWriter, request)
		return
	}
	upload, isFound := service.findReleaseUpdateUpload(uploadID)
	if !isFound || !releaseUpdateUploadTokenMatches(request, upload) {
		http.Error(responseWriter, "upload token invalid", http.StatusForbidden)
		return
	}
	chunkPath := filepath.Join(upload.DirectoryPath, "chunks", strconv.Itoa(chunkIndex))
	if errorValue := writeLimitedRequestBody(chunkPath, request.Body, releaseUpdateChunkSize); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.markReleaseUpdateUploadChunk(uploadID, chunkIndex)
	service.writeJSON(responseWriter, map[string]string{"status": "ok"})
}

func (service *Service) completeReleaseUpdateUpload(responseWriter http.ResponseWriter, request *http.Request, path string) {
	uploadID := strings.TrimSuffix(strings.TrimPrefix(path, "/updates/uploads/"), "/complete")
	upload, isFound := service.findReleaseUpdateUpload(uploadID)
	if !isFound || !releaseUpdateUploadTokenMatches(request, upload) {
		http.Error(responseWriter, "upload token invalid", http.StatusForbidden)
		return
	}
	var payload releaseUpdateUploadCompleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	archivePath := filepath.Join(upload.DirectoryPath, firstNonEmpty(upload.Filename, "release.tar.gz"))
	if errorValue := assembleChunkDirectory(upload.DirectoryPath, payload.Chunks, archivePath, "release upload"); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := validateReleaseUpdateBundleHash(archivePath, firstNonEmpty(payload.SHA256, upload.SHA256)); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	bundlePath, manifest, errorValue := service.prepareReleaseUpdateBundle(archivePath)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if job := service.activeReleaseUpdateJob(); job != nil {
		service.writeJSON(responseWriter, job)
		return
	}
	job := service.newJob("release-update")
	service.writeJSON(responseWriter, job)
	go service.runReleaseUpdateJobWithProvider(context.Background(), job.JobID, manifest, service.localReleaseComponentProvider(bundlePath))
}

func (service *Service) prepareReleaseUpdateBundle(archivePath string) (string, *releaseset.Manifest, error) {
	bundlePath := filepath.Join(service.Configuration.StateDirectory, "release-updates", "bundles", randomHex(12))
	if errorValue := os.MkdirAll(bundlePath, 0o700); errorValue != nil {
		return "", nil, errorValue
	}
	if errorValue := extractTarGzip(archivePath, bundlePath); errorValue != nil {
		return "", nil, errorValue
	}
	manifest, errorValue := service.readUploadedReleaseManifest(bundlePath)
	if errorValue != nil {
		return "", nil, errorValue
	}
	return bundlePath, manifest, nil
}

func (service *Service) readUploadedReleaseManifest(bundlePath string) (*releaseset.Manifest, error) {
	document, errorValue := os.ReadFile(filepath.Join(bundlePath, "manifest.json"))
	if errorValue != nil {
		return nil, errorValue
	}
	var manifest releaseset.Manifest
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
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

func (service *Service) localReleaseComponentProvider(bundlePath string) releaseComponentProvider {
	return func(_ context.Context, manifest *releaseset.Manifest, stagingPath string) error {
		for componentName, component := range manifest.Components {
			componentStagingPath := filepath.Join(stagingPath, componentName)
			archivePath := filepath.Join(componentStagingPath, "component.tar.gz")
			if errorValue := os.MkdirAll(componentStagingPath, 0o700); errorValue != nil {
				return errorValue
			}
			sourcePath, errorValue := releaseBundleComponentPath(bundlePath, component.BlobPath)
			if errorValue != nil {
				return errorValue
			}
			if errorValue := copyFile(sourcePath, archivePath, 0o600); errorValue != nil {
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
}

func releaseBundleComponentPath(bundlePath string, blobPath string) (string, error) {
	cleanName := filepath.Clean(strings.TrimPrefix(blobPath, "/"))
	if cleanName == "." || strings.HasPrefix(cleanName, "..") {
		return "", errors.New("release manifest contains unsafe blob path")
	}
	sourcePath := filepath.Join(bundlePath, cleanName)
	if !strings.HasPrefix(sourcePath, filepath.Clean(bundlePath)+string(os.PathSeparator)) {
		return "", errors.New("release manifest contains blob path outside bundle")
	}
	if _, errorValue := os.Stat(sourcePath); errorValue != nil {
		return "", fmt.Errorf("release component blob missing at %s", blobPath)
	}
	return sourcePath, nil
}

func validateReleaseUpdateBundleHash(path string, expectedSHA256 string) error {
	expectedSHA256 = strings.ToLower(strings.TrimSpace(expectedSHA256))
	if expectedSHA256 == "" {
		return nil
	}
	actualSHA256 := fileSHA256(path)
	if !strings.EqualFold(actualSHA256, expectedSHA256) {
		return errors.New("release upload bundle checksum mismatch")
	}
	return nil
}

func (service *Service) findReleaseUpdateUpload(uploadID string) (*ReleaseUpdateUpload, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload, isFound := service.releaseUpdateUploads[uploadID]
	return upload, isFound
}

func (service *Service) findResumableReleaseUpdateUpload(releaseID string, sha256Value string) (*ReleaseUpdateUpload, []int, bool) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	for _, upload := range service.releaseUpdateUploads {
		if upload.ReleaseID == releaseID && upload.SHA256 == sha256Value && len(upload.ReceivedChunks) > 0 {
			return upload, receivedChunkIndices(upload), true
		}
	}
	return nil, nil, false
}

func receivedChunkIndices(upload *ReleaseUpdateUpload) []int {
	indices := make([]int, 0, len(upload.ReceivedChunks))
	for index, isReceived := range upload.ReceivedChunks {
		if isReceived {
			indices = append(indices, index)
		}
	}
	sort.Ints(indices)
	return indices
}

func (service *Service) markReleaseUpdateUploadChunk(uploadID string, chunkIndex int) {
	service.mutex.Lock()
	defer service.mutex.Unlock()
	upload := service.releaseUpdateUploads[uploadID]
	if upload != nil {
		upload.ReceivedChunks[chunkIndex] = true
	}
}

func releaseUpdateUploadTokenMatches(request *http.Request, upload *ReleaseUpdateUpload) bool {
	token := strings.TrimSpace(request.Header.Get("X-InternKim-Upload-Token"))
	if token == "" {
		token = strings.TrimPrefix(strings.TrimSpace(request.Header.Get("Authorization")), "Bearer ")
	}
	return token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(upload.Token)) == 1
}

func parseReleaseUpdateUploadChunkPath(path string) (string, int, bool) {
	trimmedPath := strings.TrimPrefix(path, "/updates/uploads/")
	parts := strings.Split(trimmedPath, "/chunks/")
	if len(parts) != 2 {
		return "", 0, false
	}
	chunkIndex, errorValue := strconv.Atoi(parts[1])
	return parts[0], chunkIndex, errorValue == nil && parts[0] != "" && chunkIndex >= 0
}
