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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/blueclawworkspace"
	"gitlab.com/eastriver/internkim/internal/releaseset"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type releaseBlob struct {
	component releaseset.Component
	path      string
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
	fmt.Println("  INTERNKIM_RELEASE_R2_ACCOUNT_ID")
	fmt.Println("  INTERNKIM_RELEASE_R2_BUCKET")
	fmt.Println("  INTERNKIM_RELEASE_R2_ACCESS_KEY_ID")
	fmt.Println("  INTERNKIM_RELEASE_R2_SECRET_ACCESS_KEY")
	fmt.Println("  INTERNKIM_RELEASE_PUBLIC_BASE_URL")
	fmt.Println("  INTERNKIM_RELEASE_SIGNING_KEY optional")
}

func runReleasePublish(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	channel := firstNonEmptyString(commandArgumentValue(arguments, "--channel", ""), "stable")
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
	r2Client, errorValue := releaseR2Client()
	if errorValue != nil {
		return errorValue
	}
	for _, blob := range blobs {
		document, errorValue := os.ReadFile(blob.path)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := r2Client.PutObject(blob.component.BlobPath, document, "application/gzip"); errorValue != nil {
			return errorValue
		}
		fmt.Printf("uploaded %s %s\n", blob.component.Name, blob.component.SHA256[:12])
	}
	manifestKey := "releases/" + releaseID + "/manifest.json"
	if errorValue := r2Client.PutObject(manifestKey, append(manifestDocument, '\n'), "application/json"); errorValue != nil {
		return errorValue
	}
	pointer := releaseset.StablePointer{
		ReleaseID:   releaseID,
		ManifestURL: r2Client.PublicURL(manifestKey),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	pointerDocument, errorValue := json.MarshalIndent(pointer, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := r2Client.PutObject("channels/"+channel+".json", append(pointerDocument, '\n'), "application/json"); errorValue != nil {
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
		{name: "adminWeb", revision: adminWebRevision(repositoryRootPath), restartGroup: "admind", healthCheck: "admin-web", sourcePath: filepath.Join(repositoryRootPath, "build", "board-ui")},
		{name: "blueclawPayload", revision: blueclawPayloadRevision(repositoryRootPath), restartGroup: "blueclaw", healthCheck: "blueclaw", sourcePath: filepath.Join(repositoryRootPath, blueclaw.BlueclawPayloadArtifactPath)},
		{name: "skills", revision: gitRevision, restartGroup: "blueclaw", healthCheck: "skills", sourcePath: blueclawworkspace.SkillsPath(repositoryRootPath)},
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
		if errorValue := validateReleaseSource(input.name, input.sourcePath); errorValue != nil {
			return nil, errorValue
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
	file, errorValue := os.Create(archivePath)
	if errorValue != nil {
		return errorValue
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	walkError := filepath.WalkDir(sourcePath, func(path string, entry os.DirEntry, errorValue error) error {
		if errorValue != nil {
			return errorValue
		}
		return writeReleaseArchiveEntry(tarWriter, sourcePath, path, entry)
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
		AccountID:       os.Getenv("INTERNKIM_RELEASE_R2_ACCOUNT_ID"),
		Bucket:          os.Getenv("INTERNKIM_RELEASE_R2_BUCKET"),
		AccessKeyID:     os.Getenv("INTERNKIM_RELEASE_R2_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("INTERNKIM_RELEASE_R2_SECRET_ACCESS_KEY"),
		PublicBaseURL:   os.Getenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL"),
		HTTPClient:      statusHTTPClient,
	})
}

func fetchReleaseStablePointer(channel string) (releaseset.StablePointer, error) {
	publicBaseURL := strings.TrimRight(os.Getenv("INTERNKIM_RELEASE_PUBLIC_BASE_URL"), "/")
	if publicBaseURL == "" {
		return releaseset.StablePointer{}, errors.New("INTERNKIM_RELEASE_PUBLIC_BASE_URL is required")
	}
	response, errorValue := statusHTTPClient.Get(publicBaseURL + "/channels/" + channel + ".json")
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

func adminWebRevision(repositoryRootPath string) string {
	version, errorValue := readAdminUIVersion(filepath.Join(repositoryRootPath, "build", "board-ui"))
	if errorValue == nil && strings.TrimSpace(version) != "" {
		return strings.TrimSpace(version)
	}
	return gitRevision(repositoryRootPath)
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
