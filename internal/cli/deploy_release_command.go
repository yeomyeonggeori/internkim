package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/releaseset"
)

const releaseUpdateUploadAction = "release-update-upload"
const releaseUpdateUploadChunkSize = 4 << 20
const releaseUpdateHTTPTimeout = 2 * time.Minute

var releaseUpdateHTTPClient = &http.Client{
	Timeout: releaseUpdateHTTPTimeout,
	CheckRedirect: func(request *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type releaseUpdateUploadCreateRequest struct {
	recoveryRequest
	ReleaseID string `json:"releaseID"`
	Filename  string `json:"filename"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
}

type releaseUpdateUploadCreateResponse struct {
	UploadID    string `json:"uploadID"`
	UploadToken string `json:"uploadToken"`
	ChunkSize   int    `json:"chunkSize"`
}

type releaseUpdateUploadCompleteRequest struct {
	Chunks int    `json:"chunks"`
	SHA256 string `json:"sha256"`
}

type directReleaseBundle struct {
	path     string
	sha256   string
	size     int64
	manifest releaseset.Manifest
}

func deployUsageText() string {
	return `Usage: internkim deploy [OPTIONS]

Deploy a release to the target device over Admin HTTPS.

Options:
  --components <list>  Comma-separated component names to include.
                       Available: admind, blueclawPayload, capabilityd, internkim, skills, web
                       Example: --components admind,web
  --release <id>       Override the release ID.
  --channel <name>     Override the release channel (default: stable).
  --node <id>          Target a specific node by ID.
  --legacy-ssh         Use legacy SSH deploy path instead of OTA.
  -h, --help           Print this usage and exit.`
}

func validateDeployArguments(arguments []string) error {
	knownFlags := map[string]bool{
		"--help":        true,
		"-h":            true,
		"--components":  true,
		"--release":     true,
		"--channel":     true,
		"--node":        true,
		"--node-id":     true,
		"--host":        true,
		"--legacy-ssh":  true,
		"--all-active":  true,
		"--board":       true,
		"--board-type":  true,
		"--sim":         true,
		"--sim-name":    true,
		"--":            true,
	}
	knownValueFlags := map[string]bool{
		"--components": true,
		"--release":    true,
		"--channel":    true,
		"--node":       true,
		"--node-id":    true,
		"--host":       true,
		"--board":      true,
		"--board-type": true,
		"--sim-name":   true,
	}
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "-") {
			continue
		}
		name := argument
		if equalIndex := strings.Index(argument, "="); equalIndex != -1 {
			name = argument[:equalIndex]
		}
		if !knownFlags[name] {
			return fmt.Errorf("unrecognized flag: %s", argument)
		}
		if knownValueFlags[name] && !strings.Contains(argument, "=") {
			index++
		}
	}
	return nil
}

func runDirectReleaseDeploy(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	target := resolveCommandTarget(commandControlArguments(arguments))
	printCommandTargetEvidence(target)
	temporaryDirectoryPath, errorValue := os.MkdirTemp("", "internkim-direct-release-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(temporaryDirectoryPath)
	bundle, errorValue := createDirectReleaseBundle(repositoryRootPath, temporaryDirectoryPath, arguments)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("Release: %s\n", bundle.manifest.ReleaseID)
	fmt.Printf("Components: %s\n", strings.Join(releaseManifestComponentNames(bundle.manifest), ", "))
	fmt.Println("Creating upload session...")
	upload, errorValue := createReleaseUpdateUpload(target, bundle)
	if errorValue != nil {
		return errorValue
	}
	chunkSize := upload.ChunkSize
	if chunkSize <= 0 {
		chunkSize = releaseUpdateUploadChunkSize
	}
	fmt.Println("Uploading release bundle...")
	if errorValue := uploadReleaseUpdateBundle(target, upload, bundle.path, chunkSize); errorValue != nil {
		return errorValue
	}
	fmt.Println("Applying uploaded release...")
	job, errorValue := completeReleaseUpdateUpload(target, upload, bundle.sha256, chunkCount(bundle.size, int64(chunkSize)))
	if errorValue != nil {
		return errorValue
	}
	completedJob, errorValue := waitForReleaseUpdateJob(target, job)
	if errorValue != nil {
		return errorValue
	}
	status, statusError := fetchDeviceReleaseUpdateStatusForTarget(target)
	if statusError == nil {
		fmt.Printf("Current: %s\n", releaseUpdateID(status.Current))
	}
	fmt.Printf("Deploy: %s/%s\n", completedJob.Status, completedJob.Phase)
	return nil
}

func createDirectReleaseBundle(repositoryRootPath string, temporaryDirectoryPath string, arguments []string) (directReleaseBundle, error) {
	selectedComponentNames, errorValue := selectedReleaseComponentNames(arguments)
	if errorValue != nil {
		return directReleaseBundle{}, errorValue
	}
	blobs, errorValue := createReleaseBlobs(repositoryRootPath, temporaryDirectoryPath, selectedComponentNames)
	if errorValue != nil {
		return directReleaseBundle{}, errorValue
	}
	if len(blobs) == 0 {
		return directReleaseBundle{}, errors.New("no release components selected")
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	channel := firstNonEmptyString(commandArgumentValue(arguments, "--channel", ""), "stable")
	components := map[string]releaseset.Component{}
	for _, blob := range blobs {
		component := blob.component
		component.BlobPath = "blobs/" + component.Name + ".tar.gz"
		components[component.Name] = component
	}
	manifest, errorValue := releaseset.NewManifest(releaseID, channel, components).Sign(os.Getenv("INTERNKIM_RELEASE_SIGNING_KEY"))
	if errorValue != nil {
		return directReleaseBundle{}, errorValue
	}
	archivePath := filepath.Join(temporaryDirectoryPath, "release.tar.gz")
	if errorValue := writeDirectReleaseBundleArchive(archivePath, manifest, blobs); errorValue != nil {
		return directReleaseBundle{}, errorValue
	}
	sha256Value, size, errorValue := releaseFileSHA256AndSize(archivePath)
	if errorValue != nil {
		return directReleaseBundle{}, errorValue
	}
	return directReleaseBundle{path: archivePath, sha256: sha256Value, size: size, manifest: manifest}, nil
}

func selectedReleaseComponentNames(arguments []string) (map[string]bool, error) {
	value := commandArgumentValue(arguments, "--components", "")
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	selectedComponentNames := map[string]bool{}
	for _, componentName := range strings.Split(value, ",") {
		componentName = normalizedReleaseComponentName(strings.TrimSpace(componentName))
		if componentName != "" {
			selectedComponentNames[componentName] = true
		}
	}
	if len(selectedComponentNames) == 0 {
		return nil, errors.New("--components did not name any components")
	}
	return selectedComponentNames, nil
}

func normalizedReleaseComponentName(componentName string) string {
	switch componentName {
	case "adminWeb", "admin-web":
		return "web"
	default:
		return componentName
	}
}

func writeDirectReleaseBundleArchive(archivePath string, manifest releaseset.Manifest, blobs []releaseBlob) error {
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		return errorValue
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	manifestDocument, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue == nil {
		errorValue = writeDirectReleaseBundleBytes(tarWriter, "manifest.json", append(manifestDocument, '\n'))
	}
	for _, blob := range blobs {
		if errorValue != nil {
			break
		}
		errorValue = writeDirectReleaseBundleFile(tarWriter, "blobs/"+blob.component.Name+".tar.gz", blob.path)
	}
	closeTarError := tarWriter.Close()
	closeGzipError := gzipWriter.Close()
	closeFileError := file.Close()
	for _, candidateError := range []error{errorValue, closeTarError, closeGzipError, closeFileError} {
		if candidateError != nil {
			return candidateError
		}
	}
	return nil
}

func writeDirectReleaseBundleBytes(writer *tar.Writer, name string, document []byte) error {
	header := &tar.Header{Name: name, Mode: 0o600, Size: int64(len(document))}
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	_, errorValue := writer.Write(document)
	return errorValue
}

func writeDirectReleaseBundleFile(writer *tar.Writer, name string, path string) error {
	information, errorValue := os.Stat(path)
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := tar.FileInfoHeader(information, "")
	if errorValue != nil {
		return errorValue
	}
	header.Name = name
	if errorValue := writer.WriteHeader(header); errorValue != nil {
		return errorValue
	}
	file, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = io.Copy(writer, file)
	return errorValue
}

func createReleaseUpdateUpload(target commandTarget, bundle directReleaseBundle) (releaseUpdateUploadCreateResponse, error) {
	var response releaseUpdateUploadCreateResponse
	fleetID := strings.TrimSpace(loadState(target.stateDir, "fleet_id"))
	fleetSecret := strings.TrimSpace(loadState(target.stateDir, "fleet_secret"))
	if fleetID == "" || fleetSecret == "" {
		return response, errors.New("fleet identity is not configured in local device state")
	}
	endpointURL, errorValue := releaseDeviceEndpointURL(target, "/admin/api/updates/uploads")
	if errorValue != nil {
		return response, errorValue
	}
	payload := releaseUpdateUploadCreateRequest{
		recoveryRequest: signedRecoveryRequestPayload(fleetSecret, releaseUpdateUploadAction, fleetID),
		ReleaseID:       bundle.manifest.ReleaseID,
		Filename:        "release.tar.gz",
		Size:            bundle.size,
		SHA256:          bundle.sha256,
	}
	errorValue = postReleaseUpdateJSON(endpointURL, payload, "", &response)
	if errorValue != nil && strings.Contains(errorValue.Error(), "HTTP 404") {
		return response, errors.New("direct release upload endpoint is not available on the device; deploy admind once through setup or an existing OTA path, then retry internkim deploy")
	}
	return response, errorValue
}

func uploadReleaseUpdateBundle(target commandTarget, upload releaseUpdateUploadCreateResponse, archivePath string, chunkSize int) error {
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
		endpointURL, errorValue := releaseDeviceEndpointURL(target, fmt.Sprintf("/admin/api/updates/uploads/%s/chunks/%d", upload.UploadID, chunkIndex))
		if errorValue != nil {
			return errorValue
		}
		if errorValue := putReleaseUpdateChunk(endpointURL, upload.UploadToken, buffer[:bytesRead]); errorValue != nil {
			return errorValue
		}
		if errors.Is(readError, io.ErrUnexpectedEOF) {
			return nil
		}
	}
}

func completeReleaseUpdateUpload(target commandTarget, upload releaseUpdateUploadCreateResponse, sha256Value string, chunks int) (blueclawUpdateJobResponse, error) {
	var response blueclawUpdateJobResponse
	endpointURL, errorValue := releaseDeviceEndpointURL(target, fmt.Sprintf("/admin/api/updates/uploads/%s/complete", upload.UploadID))
	if errorValue != nil {
		return response, errorValue
	}
	payload := releaseUpdateUploadCompleteRequest{Chunks: chunks, SHA256: sha256Value}
	return response, postReleaseUpdateJSON(endpointURL, payload, upload.UploadToken, &response)
}

func waitForReleaseUpdateJob(target commandTarget, job blueclawUpdateJobResponse) (blueclawUpdateJobResponse, error) {
	for attempt := 0; attempt < 180; attempt++ {
		currentJob, errorValue := fetchDeviceReleaseUpdateJob(target, job.JobID)
		if errorValue == nil {
			fmt.Printf("Status: %s/%s\n", currentJob.Status, currentJob.Phase)
			switch currentJob.Status {
			case "completed", "already_current":
				return currentJob, nil
			case "failed":
				return currentJob, errors.New(strings.TrimSpace(currentJob.Error))
			}
		}
		time.Sleep(1500 * time.Millisecond)
	}
	return job, errors.New("release deploy did not finish before timeout")
}

func postReleaseUpdateJSON(endpointURL string, requestPayload any, token string, responseValue any) error {
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
	response, errorValue := releaseUpdateHTTPClient.Do(request)
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

func putReleaseUpdateChunk(endpointURL string, token string, document []byte) error {
	request, errorValue := http.NewRequest(http.MethodPut, endpointURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("X-InternKim-Upload-Token", token)
	attachCloudflareAccessCookie(request)
	response, errorValue := releaseUpdateHTTPClient.Do(request)
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

func releaseManifestComponentNames(manifest releaseset.Manifest) []string {
	componentNames := make([]string, 0, len(manifest.Components))
	for componentName := range manifest.Components {
		componentNames = append(componentNames, componentName)
	}
	sort.Strings(componentNames)
	return componentNames
}
