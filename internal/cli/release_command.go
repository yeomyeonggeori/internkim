package cli

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/deviceassets"
	"gitlab.com/eastriver/internkim/internal/releaseset"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type releaseBlob struct {
	component releaseset.Component
	path      string
}

type releaseObjectPublisher interface {
	PutObject(objectKey string, document []byte, contentType string) error
	DeleteObject(objectKey string) error
	PublicURL(objectKey string) string
}

type wranglerReleasePublisher struct {
	accountID     string
	bucket        string
	commandPath   string
	publicBaseURL string
}

func runRelease() {
	if len(os.Args) < 3 || os.Args[2] == "--help" || os.Args[2] == "-h" {
		printReleaseUsage()
		return
	}
	switch os.Args[2] {
	case "publish":
		if errorValue := runReleasePublish(os.Args[3:]); errorValue != nil {
			fatal(errorValue.Error())
		}
	case "status":
		if errorValue := runReleaseStatus(os.Args[3:]); errorValue != nil {
			fatal(errorValue.Error())
		}
	default:
		printReleaseUsage()
	}
}

func printReleaseUsage() {
	fmt.Println("Usage: internkim release <publish|status>")
	fmt.Println()
	fmt.Println("Environment for publish:")
	fmt.Println("  INTERNKIM_RELEASE_R2_ACCOUNT_ID falls back to CF_ACCOUNT_ID")
	fmt.Println("  INTERNKIM_RELEASE_R2_BUCKET")
	fmt.Println("  INTERNKIM_RELEASE_R2_ACCESS_KEY_ID")
	fmt.Println("  INTERNKIM_RELEASE_R2_SECRET_ACCESS_KEY")
	fmt.Println("  INTERNKIM_RELEASE_PUBLIC_BASE_URL")
	fmt.Println("  INTERNKIM_RELEASE_R2_PUBLISHER optional: s3 or wrangler")
	fmt.Println("  INTERNKIM_RELEASE_SIGNING_KEY optional")
	fmt.Println("  INTERNKIM_RELEASE_DOWNLOAD_TOKEN optional: falls back to .local/secrets/release-download-token")
}

func runReleasePublish(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	channel := firstNonEmptyString(commandArgumentValue(arguments, "--channel", ""), "stable")
	retainedReleaseCount, errorValue := releaseRetentionLimit(arguments)
	if errorValue != nil {
		return errorValue
	}
	temporaryDirectoryPath, errorValue := os.MkdirTemp("", "internkim-release-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(temporaryDirectoryPath)
	blobs, errorValue := createReleaseBlobs(repositoryRootPath, temporaryDirectoryPath, nil)
	if errorValue != nil {
		return errorValue
	}
	components := map[string]releaseset.Component{}
	for _, blob := range blobs {
		components[blob.component.Name] = blob.component
	}
	manifest, errorValue := releaseset.NewManifest(releaseID, channel, components).Sign(os.Getenv("INTERNKIM_RELEASE_SIGNING_KEY"))
	if errorValue != nil {
		return errorValue
	}
	manifestDocument, errorValue := json.MarshalIndent(manifest, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	for _, blob := range blobs {
		document, errorValue := os.ReadFile(blob.path)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := publisher.PutObject(blob.component.BlobPath, document, "application/gzip"); errorValue != nil {
			return errorValue
		}
		fmt.Printf("uploaded %s %s\n", blob.component.Name, blob.component.SHA256[:12])
	}
	manifestKey := "releases/" + releaseID + "/manifest.json"
	if errorValue := publisher.PutObject(manifestKey, append(manifestDocument, '\n'), "application/json"); errorValue != nil {
		return errorValue
	}
	pointer := releaseset.StablePointer{
		ReleaseID:   releaseID,
		ManifestURL: publisher.PublicURL(manifestKey),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	pointerDocument, errorValue := json.MarshalIndent(pointer, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := publisher.PutObject("channels/"+channel+".json", append(pointerDocument, '\n'), "application/json"); errorValue != nil {
		return errorValue
	}
	historyEntry := releaseset.ChannelHistoryEntry{
		ReleaseID:   releaseID,
		ManifestURL: pointer.ManifestURL,
		CreatedAt:   manifest.CreatedAt,
	}
	if errorValue := updateReleaseChannelHistory(publisher, channel, historyEntry, retainedReleaseCount, &manifest); errorValue != nil {
		return errorValue
	}
	fmt.Printf("published %s -> %s\n", channel, releaseID)
	return nil
}

func runReleaseStatus(arguments []string) error {
	channel := firstNonEmptyString(commandArgumentValue(arguments, "--channel", ""), "stable")
	pointer, errorValue := fetchReleaseStablePointer(channel)
	if errorValue != nil {
		return errorValue
	}
	fmt.Printf("Channel: %s\n", channel)
	fmt.Printf("Release: %s\n", pointer.ReleaseID)
	fmt.Printf("Manifest: %s\n", pointer.ManifestURL)
	return nil
}

func releaseRetentionLimit(arguments []string) (int, error) {
	value := strings.TrimSpace(commandArgumentValue(arguments, "--keep", ""))
	if value == "" {
		return 10, nil
	}
	limit, errorValue := strconv.Atoi(value)
	if errorValue != nil || limit < 1 {
		return 0, fmt.Errorf("--keep must be a positive integer, got %q", value)
	}
	return limit, nil
}

func updateReleaseChannelHistory(
	publisher releaseObjectPublisher,
	channel string,
	entry releaseset.ChannelHistoryEntry,
	retainedReleaseCount int,
	currentManifest *releaseset.Manifest,
) error {
	existingHistory, errorValue := fetchReleaseChannelHistory(publisher, channel)
	if errorValue != nil {
		return errorValue
	}
	history, prunedEntries := buildReleaseChannelHistory(existingHistory, channel, entry, retainedReleaseCount, time.Now().UTC())
	historyDocument, errorValue := json.MarshalIndent(history, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := publisher.PutObject(releaseChannelHistoryKey(channel), append(historyDocument, '\n'), "application/json"); errorValue != nil {
		return errorValue
	}
	for _, warning := range pruneReleaseChannelHistory(publisher, history.Entries, prunedEntries, currentManifest) {
		fmt.Fprintf(os.Stderr, "warning: %v\n", warning)
	}
	return nil
}

func fetchReleaseChannelHistory(publisher releaseObjectPublisher, channel string) (releaseset.ChannelHistory, error) {
	historyURL := publisher.PublicURL(releaseChannelHistoryKey(channel))
	request, errorValue := http.NewRequest(http.MethodGet, historyURL, nil)
	if errorValue != nil {
		return releaseset.ChannelHistory{}, errorValue
	}
	addReleaseDownloadHeaders(request)
	response, errorValue := statusHTTPClient.Do(request)
	if errorValue != nil {
		return releaseset.ChannelHistory{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return releaseset.ChannelHistory{Channel: channel}, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return releaseset.ChannelHistory{}, fmt.Errorf("fetch release history: HTTP %d", response.StatusCode)
	}
	var history releaseset.ChannelHistory
	if errorValue := json.NewDecoder(response.Body).Decode(&history); errorValue != nil {
		return releaseset.ChannelHistory{}, errorValue
	}
	return history, nil
}

func buildReleaseChannelHistory(
	existingHistory releaseset.ChannelHistory,
	channel string,
	entry releaseset.ChannelHistoryEntry,
	retainedReleaseCount int,
	updatedAt time.Time,
) (releaseset.ChannelHistory, []releaseset.ChannelHistoryEntry) {
	entries := []releaseset.ChannelHistoryEntry{entry}
	for _, existingEntry := range existingHistory.Entries {
		if strings.TrimSpace(existingEntry.ReleaseID) == "" || existingEntry.ReleaseID == entry.ReleaseID {
			continue
		}
		entries = append(entries, existingEntry)
	}
	retainedEntries, prunedEntries := trimReleaseHistoryEntries(entries, retainedReleaseCount)
	return releaseset.ChannelHistory{
		Channel:   channel,
		Entries:   retainedEntries,
		UpdatedAt: updatedAt.UTC().Format(time.RFC3339),
	}, prunedEntries
}

func trimReleaseHistoryEntries(entries []releaseset.ChannelHistoryEntry, retainedReleaseCount int) ([]releaseset.ChannelHistoryEntry, []releaseset.ChannelHistoryEntry) {
	if retainedReleaseCount >= len(entries) {
		return entries, nil
	}
	return entries[:retainedReleaseCount], entries[retainedReleaseCount:]
}

func pruneReleaseChannelHistory(
	publisher releaseObjectPublisher,
	retainedEntries []releaseset.ChannelHistoryEntry,
	prunedEntries []releaseset.ChannelHistoryEntry,
	currentManifest *releaseset.Manifest,
) []error {
	if len(prunedEntries) == 0 {
		return nil
	}
	retainedManifests, errorValue := fetchRetainedReleaseManifests(retainedEntries, currentManifest)
	if errorValue != nil {
		return []error{errorValue}
	}
	retainedBlobKeys := releaseBlobKeys(retainedManifests)
	warnings := []error{}
	for _, entry := range prunedEntries {
		manifest, errorValue := fetchReleaseManifestDocument(entry.ManifestURL)
		if errorValue != nil {
			warnings = append(warnings, fmt.Errorf("skip prune %s: %w", entry.ReleaseID, errorValue))
			continue
		}
		warnings = append(warnings, deletePrunedReleaseObjects(publisher, entry.ReleaseID, manifest, retainedBlobKeys)...)
	}
	return warnings
}

func fetchRetainedReleaseManifests(
	retainedEntries []releaseset.ChannelHistoryEntry,
	currentManifest *releaseset.Manifest,
) ([]releaseset.Manifest, error) {
	manifests := []releaseset.Manifest{}
	for _, entry := range retainedEntries {
		if currentManifest != nil && currentManifest.ReleaseID == entry.ReleaseID {
			manifests = append(manifests, *currentManifest)
			continue
		}
		manifest, errorValue := fetchReleaseManifestDocument(entry.ManifestURL)
		if errorValue != nil {
			return nil, fmt.Errorf("skip pruning because retained release %s could not be read: %w", entry.ReleaseID, errorValue)
		}
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

func fetchReleaseManifestDocument(manifestURL string) (releaseset.Manifest, error) {
	request, errorValue := http.NewRequest(http.MethodGet, manifestURL, nil)
	if errorValue != nil {
		return releaseset.Manifest{}, errorValue
	}
	addReleaseDownloadHeaders(request)
	response, errorValue := statusHTTPClient.Do(request)
	if errorValue != nil {
		return releaseset.Manifest{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return releaseset.Manifest{}, fmt.Errorf("fetch manifest: HTTP %d", response.StatusCode)
	}
	var manifest releaseset.Manifest
	if errorValue := json.NewDecoder(response.Body).Decode(&manifest); errorValue != nil {
		return releaseset.Manifest{}, errorValue
	}
	if errorValue := manifest.Validate(); errorValue != nil {
		return releaseset.Manifest{}, errorValue
	}
	return manifest, nil
}

func releaseBlobKeys(manifests []releaseset.Manifest) map[string]bool {
	blobKeys := map[string]bool{}
	for _, manifest := range manifests {
		for _, component := range manifest.Components {
			blobKeys[component.BlobPath] = true
		}
	}
	return blobKeys
}

func deletePrunedReleaseObjects(
	publisher releaseObjectPublisher,
	releaseID string,
	manifest releaseset.Manifest,
	retainedBlobKeys map[string]bool,
) []error {
	warnings := []error{}
	manifestKey := releaseManifestKey(releaseID)
	if errorValue := publisher.DeleteObject(manifestKey); errorValue != nil {
		warnings = append(warnings, errorValue)
	}
	for _, blobKey := range sortedBlobKeys(manifest) {
		if retainedBlobKeys[blobKey] {
			continue
		}
		if errorValue := publisher.DeleteObject(blobKey); errorValue != nil {
			warnings = append(warnings, errorValue)
		}
	}
	return warnings
}

func sortedBlobKeys(manifest releaseset.Manifest) []string {
	blobKeySet := map[string]bool{}
	for _, component := range manifest.Components {
		blobKeySet[component.BlobPath] = true
	}
	blobKeys := make([]string, 0, len(blobKeySet))
	for blobKey := range blobKeySet {
		blobKeys = append(blobKeys, blobKey)
	}
	sort.Strings(blobKeys)
	return blobKeys
}

func releaseManifestKey(releaseID string) string {
	return "releases/" + strings.Trim(strings.TrimSpace(releaseID), "/") + "/manifest.json"
}

func releaseChannelHistoryKey(channel string) string {
	return "channels/" + strings.Trim(strings.TrimSpace(channel), "/") + "-history.json"
}

func deviceAssetSourcePath(assetName string, repositoryRootPath string) string {
	asset, found := deviceassets.Find(assetName)
	if !found {
		panic("deviceassets: unknown asset " + assetName)
	}
	return asset.SourcePath(repositoryRootPath)
}

var buildBlueclawLLMDArtifact = buildBlueclawLLMDReleaseArtifact

func createReleaseBlobs(repositoryRootPath string, temporaryDirectoryPath string, selectedComponentNames map[string]bool) ([]releaseBlob, error) {
	gitRevision := gitRevision(repositoryRootPath)
	blobInputs := []struct {
		name         string
		revision     string
		restartGroup string
		healthCheck  string
		sourcePath   string
		builder      func(string, string) error
	}{
		{name: "internkim", revision: gitRevision, restartGroup: "admind", healthCheck: "binary", sourcePath: filepath.Join(temporaryDirectoryPath, "bin", "internkim"), builder: buildReleaseBinary("./cmd/internkim")},
		{name: "admind", revision: gitRevision, restartGroup: "admind", healthCheck: "admind", sourcePath: filepath.Join(temporaryDirectoryPath, "bin", blueclaw.AdmindName), builder: buildReleaseBinary("./cmd/" + blueclaw.AdmindName)},
		{name: "capabilityd", revision: gitRevision, restartGroup: "capabilityd", healthCheck: "capabilityd", sourcePath: filepath.Join(temporaryDirectoryPath, "bin", blueclaw.CapabilitydName), builder: buildReleaseBinary("./cmd/" + blueclaw.CapabilitydName)},
		{name: "web", revision: webRevision(repositoryRootPath), restartGroup: "admind", healthCheck: "web", sourcePath: filepath.Join(repositoryRootPath, "build", "board-ui")},
		{name: "blueclawLLMD", restartGroup: "blueclaw", healthCheck: "blueclawLLMD", sourcePath: filepath.Join(repositoryRootPath, ".dependency", "blueclaw-llmd"), builder: buildBlueclawLLMDArtifact},
		{name: "blueclawPayload", revision: blueclawPayloadRevision(repositoryRootPath), restartGroup: "blueclaw", healthCheck: "blueclaw", sourcePath: filepath.Join(repositoryRootPath, blueclaw.BlueclawPayloadArtifactPath)},
		{name: "blueclawSupervisor", revision: gitRevision, restartGroup: "blueclaw", healthCheck: "blueclaw", sourcePath: filepath.Join(temporaryDirectoryPath, "bin", blueclaw.BlueclawSupervisorName), builder: buildBlueclawSupervisorReleaseBinary},
		{name: "skills", revision: gitRevision, restartGroup: "blueclaw", healthCheck: "skills", sourcePath: deviceAssetSourcePath("skills", repositoryRootPath)},
		{name: "fonts", revision: gitRevision, restartGroup: "admind", healthCheck: "web", sourcePath: deviceAssetSourcePath("fonts", repositoryRootPath)},
		{name: "mattermostPlugins", revision: gitRevision, restartGroup: "admind", healthCheck: "mattermostPlugins", sourcePath: filepath.Join(repositoryRootPath, "build", "mattermost-plugins")},
		{name: "chatd", revision: gitRevision, restartGroup: "chatd", healthCheck: "binary", sourcePath: filepath.Join(temporaryDirectoryPath, "bin", blueclaw.ChatdName), builder: buildChatdReleaseBinary},
		{name: "buzzMigrate", revision: gitRevision, restartGroup: "", healthCheck: "binary", sourcePath: filepath.Join(temporaryDirectoryPath, "bin", blueclaw.BuzzMigrateName), builder: buildReleaseBinary("./cmd/buzz-migrate")},
	}
	blobs := []releaseBlob{}
	for _, input := range blobInputs {
		if len(selectedComponentNames) > 0 && !selectedComponentNames[input.name] {
			continue
		}
		if input.builder != nil {
			if errorValue := input.builder(repositoryRootPath, input.sourcePath); errorValue != nil {
				return nil, errorValue
			}
		}
		if input.name == "blueclawLLMD" {
			input.revision = blueclawLLMDRevision(repositoryRootPath)
		}
		if errorValue := validateReleaseSource(input.name, input.sourcePath); errorValue != nil {
			return nil, errorValue
		}
		if input.name == "blueclawLLMD" {
			if errorValue := validateBlueclawLLMDFreshness(repositoryRootPath, input.sourcePath); errorValue != nil {
				return nil, errorValue
			}
		}
		if input.name == "blueclawPayload" {
			if errorValue := validateBlueclawPayloadFreshness(repositoryRootPath, input.sourcePath); errorValue != nil {
				return nil, errorValue
			}
		}
		archivePath := filepath.Join(temporaryDirectoryPath, input.name+".tar.gz")
		if errorValue := writeReleaseArchive(input.sourcePath, archivePath); errorValue != nil {
			return nil, errorValue
		}
		sha256Value, size, errorValue := releaseFileSHA256AndSize(archivePath)
		if errorValue != nil {
			return nil, errorValue
		}
		blobs = append(blobs, releaseBlob{
			component: releaseset.Component{
				Name:         input.name,
				Revision:     input.revision,
				SHA256:       sha256Value,
				Size:         size,
				BlobPath:     "blobs/sha256/" + sha256Value,
				RestartGroup: input.restartGroup,
				HealthCheck:  input.healthCheck,
			},
			path: archivePath,
		})
	}
	return blobs, nil
}

func buildChatdReleaseBinary(repositoryRootPath string, outputPath string) error {
	if errorValue := os.MkdirAll(filepath.Dir(outputPath), 0o755); errorValue != nil {
		return errorValue
	}
	command := exec.Command("bun", "build", "src/main.ts", "--compile", "--target=bun-linux-arm64", "--outfile", outputPath)
	command.Dir = filepath.Join(repositoryRootPath, ".dependency", "blueclaw", "chatd")
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("build chatd: %s", strings.TrimSpace(string(output)))
	}
	return nil
}

func buildReleaseBinary(packagePath string) func(string, string) error {
	return func(repositoryRootPath string, outputPath string) error {
		if errorValue := os.MkdirAll(filepath.Dir(outputPath), 0o755); errorValue != nil {
			return errorValue
		}
		command := exec.Command("go", "build", "-o", outputPath, packagePath)
		command.Dir = repositoryRootPath
		command.Env = append(os.Environ(), "GOOS=linux", "GOARCH=arm64")
		output, errorValue := command.CombinedOutput()
		if errorValue != nil {
			return fmt.Errorf("build %s: %s", packagePath, strings.TrimSpace(string(output)))
		}
		return nil
	}
}

func buildBlueclawLLMDReleaseArtifact(repositoryRootPath string, outputPath string) error {
	command := exec.Command(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-llmd"))
	command.Dir = repositoryRootPath
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("build blueclaw LLMD: %s", strings.TrimSpace(string(output)))
	}
	return validateReleaseSource("blueclawLLMD", outputPath)
}

func buildBlueclawSupervisorReleaseBinary(repositoryRootPath string, outputPath string) error {
	return blueclaw.EnsureBlueclawSupervisorBinary(outputPath, repositoryRootPath)
}

func validateBlueclawLLMDFreshness(repositoryRootPath string, artifactDirectoryPath string) error {
	document, errorValue := os.ReadFile(filepath.Join(artifactDirectoryPath, "manifest.json"))
	if errorValue != nil {
		return fmt.Errorf("read blueclaw LLMD manifest: %w", errorValue)
	}
	var manifest struct {
		BlueclawRevision string `json:"blueclawRevision"`
		SHA256           string `json:"sha256"`
	}
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return fmt.Errorf("parse blueclaw LLMD manifest: %w", errorValue)
	}
	expectedRevision, errorValue := blueclawLLMDSourceRevision(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(manifest.BlueclawRevision) != expectedRevision {
		return fmt.Errorf("blueclaw LLMD artifact revision %q does not match current protocol and LLMD source %q; run `make prepare-blueclaw-llmd`", manifest.BlueclawRevision, expectedRevision)
	}
	binarySHA256, _, errorValue := releaseFileSHA256AndSize(filepath.Join(artifactDirectoryPath, blueclaw.LLMDName))
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(manifest.SHA256) != binarySHA256 {
		return errors.New("blueclaw LLMD artifact checksum does not match its manifest")
	}
	return nil
}

func blueclawLLMDSourceRevision(repositoryRootPath string) (string, error) {
	command := exec.Command(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-llmd"), "--print-source-revision")
	command.Dir = repositoryRootPath
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return "", fmt.Errorf("resolve blueclaw LLMD source revision: %s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	revision := strings.TrimSpace(string(output))
	if revision == "" {
		return "", errors.New("resolve blueclaw LLMD source revision: empty revision")
	}
	return revision, nil
}

func validateBlueclawPayloadFreshness(repositoryRootPath string, artifactDirectoryPath string) error {
	manifest, errorValue := blueclaw.ValidatePayloadArtifactDirectory(artifactDirectoryPath)
	if errorValue != nil {
		return errorValue
	}
	return blueclaw.ValidatePayloadArtifactSource(repositoryRootPath, manifest)
}

func validateReleaseSource(name string, sourcePath string) error {
	information, errorValue := os.Stat(sourcePath)
	if errorValue != nil {
		return fmt.Errorf("%s release source missing at %s", name, sourcePath)
	}
	if information.IsDir() {
		return nil
	}
	if information.Mode().IsRegular() {
		return nil
	}
	return fmt.Errorf("%s release source is not a regular file or directory", name)
}

func writeReleaseArchive(sourcePath string, archivePath string) error {
	archiveSourcePath, errorValue := releaseArchiveSourcePath(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		return errorValue
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	walkError := filepath.WalkDir(archiveSourcePath, func(path string, entry os.DirEntry, errorValue error) error {
		if errorValue != nil {
			return errorValue
		}
		return writeReleaseArchiveEntry(tarWriter, archiveSourcePath, path, entry)
	})
	closeTarError := tarWriter.Close()
	closeGzipError := gzipWriter.Close()
	closeFileError := file.Close()
	for _, errorValue := range []error{walkError, closeTarError, closeGzipError, closeFileError} {
		if errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func releaseArchiveSourcePath(sourcePath string) (string, error) {
	information, errorValue := os.Stat(sourcePath)
	if errorValue != nil {
		return "", errorValue
	}
	if !information.IsDir() {
		return sourcePath, nil
	}
	resolvedPath, errorValue := filepath.EvalSymlinks(sourcePath)
	if errorValue != nil {
		return "", errorValue
	}
	return resolvedPath, nil
}

func writeReleaseArchiveEntry(writer *tar.Writer, sourcePath string, currentPath string, entry os.DirEntry) error {
	information, errorValue := entry.Info()
	if errorValue != nil {
		return errorValue
	}
	header, errorValue := tar.FileInfoHeader(information, "")
	if errorValue != nil {
		return errorValue
	}
	relativePath, errorValue := filepath.Rel(filepath.Dir(sourcePath), currentPath)
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
	file, errorValue := os.Open(currentPath)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	_, errorValue = io.Copy(writer, file)
	return errorValue
}

func releaseR2Client() (releaseset.R2Client, error) {
	return releaseset.NewR2Client(releaseset.R2Configuration{
		AccountID:       releaseR2AccountID(),
		Bucket:          os.Getenv("INTERNKIM_RELEASE_R2_BUCKET"),
		AccessKeyID:     os.Getenv("INTERNKIM_RELEASE_R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("INTERNKIM_RELEASE_R2_SECRET_ACCESS_KEY"),
		PublicBaseURL:   os.Getenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL"),
		HTTPClient:      statusHTTPClient,
	})
}

// R2 shares the same Cloudflare account as the rest of the workspace, so CF_ACCOUNT_ID is a valid fallback.
func releaseR2AccountID() string {
	if accountID := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_R2_ACCOUNT_ID")); accountID != "" {
		return accountID
	}
	return strings.TrimSpace(os.Getenv("CF_ACCOUNT_ID"))
}

func releasePublisherFromEnvironment(repositoryRootPath string) (releaseObjectPublisher, error) {
	publisherName := strings.ToLower(strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_R2_PUBLISHER")))
	if publisherName == "" || publisherName == "s3" {
		return releaseR2Client()
	}
	if publisherName == "wrangler" {
		return releaseWranglerPublisher(repositoryRootPath)
	}
	return nil, fmt.Errorf("unsupported INTERNKIM_RELEASE_R2_PUBLISHER %q", publisherName)
}

func releaseWranglerPublisher(repositoryRootPath string) (wranglerReleasePublisher, error) {
	bucket := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_R2_BUCKET"))
	if bucket == "" {
		return wranglerReleasePublisher{}, errors.New("INTERNKIM_RELEASE_R2_BUCKET is required")
	}
	publicBaseURL := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL"))
	if publicBaseURL == "" {
		return wranglerReleasePublisher{}, errors.New("INTERNKIM_RELEASE_PUBLIC_BASE_URL is required")
	}
	return wranglerReleasePublisher{
		accountID:     releaseR2AccountID(),
		bucket:        bucket,
		commandPath:   releaseWranglerCommandPath(repositoryRootPath),
		publicBaseURL: publicBaseURL,
	}, nil
}

func releaseWranglerCommandPath(repositoryRootPath string) string {
	if commandPath := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_WRANGLER_BIN")); commandPath != "" {
		return commandPath
	}
	localCommandPath := filepath.Join(repositoryRootPath, "web", "node_modules", ".bin", "wrangler")
	if information, errorValue := os.Stat(localCommandPath); errorValue == nil && !information.IsDir() {
		return localCommandPath
	}
	return "wrangler"
}

func (publisher wranglerReleasePublisher) PublicURL(objectKey string) string {
	return strings.TrimRight(strings.TrimSpace(publisher.publicBaseURL), "/") + "/" + strings.TrimLeft(filepath.ToSlash(objectKey), "/")
}

func (publisher wranglerReleasePublisher) PutObject(objectKey string, document []byte, contentType string) error {
	temporaryDirectoryPath, errorValue := os.MkdirTemp("", "internkim-r2-object-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(temporaryDirectoryPath)
	objectDocumentPath := filepath.Join(temporaryDirectoryPath, "object")
	if errorValue := os.WriteFile(objectDocumentPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	arguments := wranglerObjectPutArguments(publisher.bucket, objectKey, objectDocumentPath, contentType)
	command := exec.Command(publisher.commandPath, arguments...)
	command.Dir = temporaryDirectoryPath
	command.Env = wranglerObjectPutEnvironment(os.Environ(), publisher.accountID)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("wrangler R2 put object %s failed: %s", objectKey, strings.TrimSpace(string(output)))
	}
	return nil
}

func (publisher wranglerReleasePublisher) DeleteObject(objectKey string) error {
	arguments := wranglerObjectDeleteArguments(publisher.bucket, objectKey)
	command := exec.Command(publisher.commandPath, arguments...)
	command.Env = wranglerObjectPutEnvironment(os.Environ(), publisher.accountID)
	output, errorValue := command.CombinedOutput()
	if errorValue != nil {
		return fmt.Errorf("wrangler R2 delete object %s failed: %s", objectKey, strings.TrimSpace(string(output)))
	}
	return nil
}

func wranglerObjectPutArguments(bucket string, objectKey string, filePath string, contentType string) []string {
	arguments := []string{
		"r2", "object", "put", strings.TrimSpace(bucket) + "/" + strings.TrimLeft(filepath.ToSlash(objectKey), "/"),
		"--file", filePath,
		"--force",
		"--remote",
	}
	if strings.TrimSpace(contentType) != "" {
		arguments = append(arguments, "--content-type", strings.TrimSpace(contentType))
	}
	return arguments
}

func wranglerObjectDeleteArguments(bucket string, objectKey string) []string {
	return []string{
		"r2", "object", "delete", strings.TrimSpace(bucket) + "/" + strings.TrimLeft(filepath.ToSlash(objectKey), "/"),
		"--remote",
	}
}

func wranglerObjectPutEnvironment(environment []string, accountID string) []string {
	filteredEnvironment := []string{}
	apiToken := ""
	for _, value := range environment {
		name, variableValue, _ := strings.Cut(value, "=")
		if name == "CF_API_TOKEN" || name == "CLOUDFLARE_API_TOKEN" {
			if strings.TrimSpace(variableValue) != "" {
				apiToken = strings.TrimSpace(variableValue)
			}
			continue
		}
		if name == "CF_ACCOUNT_ID" || name == "CLOUDFLARE_ACCOUNT_ID" {
			continue
		}
		filteredEnvironment = append(filteredEnvironment, value)
	}
	if apiToken != "" {
		filteredEnvironment = append(filteredEnvironment, "CLOUDFLARE_API_TOKEN="+apiToken)
	}
	if strings.TrimSpace(accountID) != "" {
		filteredEnvironment = append(filteredEnvironment, "CLOUDFLARE_ACCOUNT_ID="+strings.TrimSpace(accountID))
	}
	return filteredEnvironment
}

func fetchReleaseStablePointer(channel string) (releaseset.StablePointer, error) {
	publicBaseURL := strings.TrimRight(os.Getenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL"), "/")
	if publicBaseURL == "" {
		return releaseset.StablePointer{}, errors.New("INTERNKIM_RELEASE_PUBLIC_BASE_URL is required")
	}
	request, errorValue := http.NewRequest(http.MethodGet, publicBaseURL+"/channels/"+channel+".json", nil)
	if errorValue != nil {
		return releaseset.StablePointer{}, errorValue
	}
	addReleaseDownloadHeaders(request)
	response, errorValue := statusHTTPClient.Do(request)
	if errorValue != nil {
		return releaseset.StablePointer{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return releaseset.StablePointer{}, fmt.Errorf("release status failed: HTTP %d", response.StatusCode)
	}
	var pointer releaseset.StablePointer
	if errorValue := json.NewDecoder(response.Body).Decode(&pointer); errorValue != nil {
		return releaseset.StablePointer{}, errorValue
	}
	return pointer, nil
}

func addReleaseDownloadHeaders(request *http.Request) {
	token := releaseDownloadToken()
	if token == "" {
		return
	}
	request.Header.Set("X-InternKim-Release-Token", token)
}

// Falls back to the local secrets file so a developer with a repo checkout
// does not need to export INTERNKIM_RELEASE_DOWNLOAD_TOKEN by hand.
func releaseDownloadToken() string {
	if token := strings.TrimSpace(os.Getenv("INTERNKIM_RELEASE_DOWNLOAD_TOKEN")); token != "" {
		return token
	}
	document, errorValue := os.ReadFile(".local/secrets/release-download-token")
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func defaultReleaseID(repositoryRootPath string) string {
	return time.Now().UTC().Format("20060102T150405Z") + "-" + shortRevision(gitRevision(repositoryRootPath))
}

func gitRevision(repositoryRootPath string) string {
	return firstNonEmptyString(
		strings.TrimSpace(runCmd("git", "-C", repositoryRootPath, "rev-parse", "HEAD")),
		"unknown",
	)
}

func shortRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if len(revision) <= 12 {
		return revision
	}
	return revision[:12]
}

func webRevision(repositoryRootPath string) string {
	version, errorValue := readAdminUIVersion(filepath.Join(repositoryRootPath, "build", "board-ui"))
	if errorValue == nil && strings.TrimSpace(version) != "" {
		return strings.TrimSpace(version)
	}
	return gitRevision(repositoryRootPath)
}

func blueclawLLMDRevision(repositoryRootPath string) string {
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, ".dependency", "blueclaw-llmd", "manifest.json"))
	if errorValue != nil {
		return blueclawPayloadRevision(repositoryRootPath)
	}
	var manifest struct {
		BlueclawRevision string `json:"blueclawRevision"`
	}
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return blueclawPayloadRevision(repositoryRootPath)
	}
	return firstNonEmptyString(manifest.BlueclawRevision, blueclawPayloadRevision(repositoryRootPath))
}

func blueclawPayloadRevision(repositoryRootPath string) string {
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, blueclaw.BlueclawPayloadArtifactPath, "manifest.json"))
	if errorValue != nil {
		return gitRevision(repositoryRootPath)
	}
	var manifest struct {
		BlueclawRevision string `json:"blueclawRevision"`
	}
	if errorValue := json.Unmarshal(document, &manifest); errorValue != nil {
		return gitRevision(repositoryRootPath)
	}
	return firstNonEmptyString(manifest.BlueclawRevision, gitRevision(repositoryRootPath))
}

func releaseFileSHA256AndSize(path string) (string, int64, error) {
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

func readAllLimited(reader io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, limit))
}
