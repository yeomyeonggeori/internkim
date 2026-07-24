package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const blueclawUpdateUploadAction = "blueclaw-update-upload"
const blueclawUpdateUploadChunkSize = 4 << 20
const blueclawUpdateHTTPTimeout = 2 * time.Minute

var (
	blueclawUpdateHTTPClient = &http.Client{
		Timeout: blueclawUpdateHTTPTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	cloudflareAccessTokenByHost = map[string]string{}
)

type blueclawUpdateUploadCreateRequest struct {
	recoveryRequest
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
	Apply  bool   `json:"apply"`
}

type blueclawUpdateJobResponse struct {
	JobID  string            `json:"jobID"`
	Type   string            `json:"type"`
	Status string            `json:"status"`
	Phase  string            `json:"phase"`
	Error  string            `json:"error"`
	Result map[string]string `json:"result"`
}

type blueclawPayloadInstallOutcome struct {
	Summary              string
	AlreadyCurrent       bool
	WorkspaceImageSynced bool
}

func (state *setupFlowState) installBlueclawPayloadHTTPS(artifactDirectoryPath string, manifest blueclaw.PayloadArtifactManifest) (blueclawPayloadInstallOutcome, error) {
	if strings.TrimSpace(state.targetDeviceURL()) == "" {
		return blueclawPayloadInstallOutcome{}, errors.New("device URL is not configured")
	}
	archivePath, removeArchive, errorValue := createBlueclawPayloadArchive(artifactDirectoryPath)
	if errorValue != nil {
		return blueclawPayloadInstallOutcome{}, errorValue
	}
	defer removeArchive()
	archiveSHA256, archiveSize, errorValue := fileSHA256AndSize(archivePath)
	if errorValue != nil {
		return blueclawPayloadInstallOutcome{}, errorValue
	}
	upload, errorValue := state.createBlueclawPayloadUpload(manifest.BlueclawRevision, archiveSize, archiveSHA256)
	if errorValue != nil {
		return blueclawPayloadInstallOutcome{}, errorValue
	}
	chunkSize := upload.ChunkSize
	if chunkSize <= 0 {
		chunkSize = blueclawUpdateUploadChunkSize
	}
	if errorValue := uploadBlueclawPayloadArchive(state.targetDeviceURL(), upload, archivePath, chunkSize); errorValue != nil {
		return blueclawPayloadInstallOutcome{}, errorValue
	}
	job, errorValue := completeBlueclawPayloadUpload(state.targetDeviceURL(), upload, archiveSHA256, chunkCount(archiveSize, int64(chunkSize)))
	if errorValue != nil {
		return blueclawPayloadInstallOutcome{}, errorValue
	}
	switch job.Status {
	case "completed":
		return blueclawPayloadInstallOutcome{Summary: "installed", WorkspaceImageSynced: true}, nil
	case "already_current":
		return alreadyCurrentBlueclawPayloadInstallOutcome(job), nil
	default:
		return blueclawPayloadInstallOutcome{}, fmt.Errorf("blueclaw self-update failed at %s: %s", firstNonEmptyString(job.Phase, job.Status), strings.TrimSpace(job.Error))
	}
}

func alreadyCurrentBlueclawPayloadInstallOutcome(job blueclawUpdateJobResponse) blueclawPayloadInstallOutcome {
	if job.Result["workspaceImageSynced"] == "true" {
		return blueclawPayloadInstallOutcome{
			Summary:              "already current (workspace image synced)",
			AlreadyCurrent:       true,
			WorkspaceImageSynced: true,
		}
	}
	return blueclawPayloadInstallOutcome{
		Summary:        "already current",
		AlreadyCurrent: true,
	}
}

func (state *setupFlowState) targetDeviceURL() string {
	return strings.TrimSpace(loadState(state.stateDir, "device_url"))
}

func (state *setupFlowState) createBlueclawPayloadUpload(version string, size int64, sha256Value string) (blueclawUpdateUploadCreateResponse, error) {
	var response blueclawUpdateUploadCreateResponse
	fleetID := strings.TrimSpace(loadState(state.stateDir, "fleet_id"))
	fleetSecret := strings.TrimSpace(loadState(state.stateDir, "fleet_secret"))
	if fleetID == "" || fleetSecret == "" {
		return response, errors.New("fleet identity is not configured in local device state")
	}
	endpointURL, errorValue := publicEndpointURL(state.targetDeviceURL(), "/admin/api/updates/blueclaw/uploads")
	if errorValue != nil {
		return response, errorValue
	}
	requestPayload := blueclawUpdateUploadCreateRequest{
		recoveryRequest: signedRecoveryRequestPayload(fleetSecret, blueclawUpdateUploadAction, fleetID),
		Version:         version,
		Filename:        "blueclaw-payload.tar.gz",
		Size:            size,
		SHA256:          sha256Value,
	}
	return response, postBlueclawUpdateJSON(endpointURL, requestPayload, "", &response)
}

func uploadBlueclawPayloadArchive(deviceURL string, upload blueclawUpdateUploadCreateResponse, archivePath string, chunkSize int) error {
	file, errorValue := os.Open(archivePath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	buffer := make([]byte, chunkSize)
	for chunkIndex := 0; ; chunkIndex++ {
		bytesRead, readError := io.ReadFull(file, buffer)
		if errors.Is(readError, io.EOF) {
			return nil
		}
		if readError != nil && !errors.Is(readError, io.ErrUnexpectedEOF) {
			return readError
		}
		endpointURL, errorValue := publicEndpointURL(deviceURL, fmt.Sprintf("/admin/api/updates/blueclaw/uploads/%s/chunks/%d", upload.UploadID, chunkIndex))
		if errorValue != nil {
			return errorValue
		}
		if errorValue := putBlueclawUpdateChunk(endpointURL, upload.UploadToken, buffer[:bytesRead]); errorValue != nil {
			return errorValue
		}
		if errors.Is(readError, io.ErrUnexpectedEOF) {
			return nil
		}
	}
}

func completeBlueclawPayloadUpload(deviceURL string, upload blueclawUpdateUploadCreateResponse, sha256Value string, chunks int) (blueclawUpdateJobResponse, error) {
	var response blueclawUpdateJobResponse
	endpointURL, errorValue := publicEndpointURL(deviceURL, fmt.Sprintf("/admin/api/updates/blueclaw/uploads/%s/complete", upload.UploadID))
	if errorValue != nil {
		return response, errorValue
	}
	requestPayload := blueclawUpdateUploadCompleteRequest{
		Chunks: chunks,
		SHA256: sha256Value,
		Apply:  true,
	}
	return response, postBlueclawUpdateJSON(endpointURL, requestPayload, upload.UploadToken, &response)
}

func postBlueclawUpdateJSON(endpointURL string, requestPayload any, token string, responseValue any) error {
	document, errorValue := json.Marshal(requestPayload)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequest(http.MethodPost, endpointURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("X-InternKim-Upload-Token", token)
	}
	attachCloudflareAccessCookie(request)
	response, errorValue := blueclawUpdateHTTPClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, recoveryResponseBodyLimitBytes))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return json.NewDecoder(bytes.NewReader(responseBody)).Decode(responseValue)
}

func putBlueclawUpdateChunk(endpointURL string, token string, document []byte) error {
	request, errorValue := http.NewRequest(http.MethodPut, endpointURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("X-InternKim-Upload-Token", token)
	attachCloudflareAccessCookie(request)
	response, errorValue := blueclawUpdateHTTPClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	responseBody, _ := io.ReadAll(io.LimitReader(response.Body, recoveryResponseBodyLimitBytes))
	return fmt.Errorf("HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(responseBody)))
}

func createBlueclawPayloadArchive(artifactDirectoryPath string) (string, func(), error) {
	file, errorValue := os.CreateTemp("", "internkim-blueclaw-payload-*.tar.gz")
	if errorValue != nil {
		return "", func() {}, errorValue
	}
	archivePath := file.Name()
	removeArchive := func() { _ = os.Remove(archivePath) }
	errorValue = writeTarGzipDirectory(file, artifactDirectoryPath)
	closeError := file.Close()
	if errorValue != nil {
		removeArchive()
		return "", func() {}, errorValue
	}
	if closeError != nil {
		removeArchive()
		return "", func() {}, closeError
	}
	return archivePath, removeArchive, nil
}

func writeTarGzipDirectory(writer io.Writer, directoryPath string) error {
	gzipWriter := gzip.NewWriter(writer)
	tarWriter := tar.NewWriter(gzipWriter)
	walkError := filepath.WalkDir(directoryPath, func(path string, entry os.DirEntry, errorValue error) error {
		if errorValue != nil {
			return errorValue
		}
		return writeTarEntry(tarWriter, directoryPath, path, entry)
	})
	closeTarError := tarWriter.Close()
	closeGzipError := gzipWriter.Close()
	if walkError != nil {
		return walkError
	}
	if closeTarError != nil {
		return closeTarError
	}
	return closeGzipError
}

func writeTarEntry(writer *tar.Writer, directoryPath string, path string, entry os.DirEntry) error {
	if path == directoryPath {
		return nil
	}
	information, errorValue := entry.Info()
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := tar.FileInfoHeader(information, "")
	if errorValue != nil {
		return errorValue
	}
	relativePath, errorValue := filepath.Rel(directoryPath, path)
	if errorValue != nil {
		return errorValue
	}
	header.Name = filepath.ToSlash(relativePath)
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	if entry.IsDir() {
		return nil
	}
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = io.Copy(writer, file)
	return errorValue
}

func attachCloudflareAccessCookie(request *http.Request) {
	token := cloudflareAccessToken(request.URL.String())
	if token == "" {
		return
	}
	request.AddCookie(&http.Cookie{Name: "CF_Authorization", Value: token})
}

func cloudflareAccessToken(applicationURL string) string {
	parsedURL, errorValue := url.Parse(applicationURL)
	if errorValue != nil {
		return ""
	}
	host := parsedURL.Host
	if token := strings.TrimSpace(cloudflareAccessTokenByHost[host]); token != "" {
		return token
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "cloudflared", "access", "token", "--app="+applicationURL)
	output, errorValue := command.Output()
	if errorValue != nil {
		return ""
	}
	token := strings.TrimSpace(string(output))
	if token != "" {
		cloudflareAccessTokenByHost[host] = token
	}
	return token
}

func fileSHA256AndSize(path string) (string, int64, error) {
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return "", 0, errorValue
	}
	defer file.Close()
	hash := sha256.New()
	size, errorValue := io.Copy(hash, file)
	if errorValue != nil {
		return "", 0, errorValue
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func chunkCount(size int64, chunkSize int64) int {
	if size <= 0 || chunkSize <= 0 {
		return 0
	}
	return int((size + chunkSize - 1) / chunkSize)
}
