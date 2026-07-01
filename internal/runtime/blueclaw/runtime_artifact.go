package blueclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type RuntimeArtifactManifest struct {
	RuntimeName         string                        `json:"runtimeName"`
	Platform            string                        `json:"platform"`
	Version             string                        `json:"version"`
	GuestInitSHA256     string                        `json:"guestInitSHA256,omitempty"`
	PrepareScriptSHA256 string                        `json:"prepareScriptSHA256,omitempty"`
	BaseSourceSHA256    string                        `json:"baseSourceSHA256,omitempty"`
	Files               []RuntimeArtifactManifestFile `json:"files"`
}

type RuntimeArtifactManifestFile struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Mode   string `json:"mode"`
}

func ValidateRuntimeArtifactDirectory(artifactDirectoryPath string) (RuntimeArtifactManifest, error) {
	manifestPath := filepath.Join(artifactDirectoryPath, "manifest.json")
	manifestDocument, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return RuntimeArtifactManifest{}, fmt.Errorf("read Blueclaw runtime manifest: %w", errorValue)
	}

	manifest, errorValue := ParseRuntimeArtifactManifest(manifestDocument)
	if errorValue != nil {
		return RuntimeArtifactManifest{}, errorValue
	}
	return validateRuntimeArtifactDirectory(artifactDirectoryPath, manifest)
}

func ParseRuntimeArtifactManifest(manifestDocument []byte) (RuntimeArtifactManifest, error) {
	var manifest RuntimeArtifactManifest
	if errorValue := json.Unmarshal(manifestDocument, &manifest); errorValue != nil {
		return RuntimeArtifactManifest{}, fmt.Errorf("parse Blueclaw runtime manifest: %w", errorValue)
	}
	if strings.TrimSpace(manifest.RuntimeName) != "internkim-blueclaw-runtime" {
		return RuntimeArtifactManifest{}, fmt.Errorf("unexpected Blueclaw runtime name %q", manifest.RuntimeName)
	}
	if strings.TrimSpace(manifest.Platform) != "linux-arm64" {
		return RuntimeArtifactManifest{}, fmt.Errorf("unexpected Blueclaw runtime platform %q", manifest.Platform)
	}
	return manifest, nil
}

func validateRuntimeArtifactDirectory(artifactDirectoryPath string, manifest RuntimeArtifactManifest) (RuntimeArtifactManifest, error) {
	requiredFileNames := map[string]bool{
		"firecracker": true,
		"jailer":      true,
		"vmlinux.bin": true,
		"rootfs.ext4": true,
	}
	for _, manifestFile := range manifest.Files {
		if !requiredFileNames[manifestFile.Name] {
			continue
		}
		delete(requiredFileNames, manifestFile.Name)
		if errorValue := validateRuntimeArtifactFile(artifactDirectoryPath, manifestFile); errorValue != nil {
			return RuntimeArtifactManifest{}, errorValue
		}
	}
	if len(requiredFileNames) > 0 {
		var missingFileNames []string
		for fileName := range requiredFileNames {
			missingFileNames = append(missingFileNames, fileName)
		}
		return RuntimeArtifactManifest{}, fmt.Errorf("Blueclaw runtime manifest is missing required files: %s", strings.Join(missingFileNames, ", "))
	}

	return manifest, nil
}

func ValidateRuntimeArtifactSource(repositoryRootPath string, manifest RuntimeArtifactManifest) error {
	expectedManifest, errorValue := ExpectedRuntimeArtifactSource(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	if manifest.GuestInitSHA256 != expectedManifest.GuestInitSHA256 {
		return fmt.Errorf("Blueclaw runtime base guest-init is stale; run `make prepare-blueclaw-runtime-base`")
	}
	if manifest.PrepareScriptSHA256 != expectedManifest.PrepareScriptSHA256 {
		return fmt.Errorf("Blueclaw runtime base prepare script is stale; run `make prepare-blueclaw-runtime-base`")
	}
	if manifest.BaseSourceSHA256 != expectedManifest.BaseSourceSHA256 {
		return fmt.Errorf("Blueclaw runtime base helper source is stale; run `make prepare-blueclaw-runtime-base`")
	}
	return nil
}

func ExpectedRuntimeArtifactSource(repositoryRootPath string) (RuntimeArtifactManifest, error) {
	guestInitSHA256, errorValue := calculateFileSHA256(filepath.Join(repositoryRootPath, "assets", "blueclaw-runtime", "guest-init"))
	if errorValue != nil {
		return RuntimeArtifactManifest{}, fmt.Errorf("hash Blueclaw guest init: %w", errorValue)
	}
	prepareScriptSHA256, errorValue := calculateFileSHA256(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		return RuntimeArtifactManifest{}, fmt.Errorf("hash Blueclaw runtime prepare script: %w", errorValue)
	}
	baseSourceSHA256, errorValue := calculateRuntimeBaseSourceSHA256(repositoryRootPath)
	if errorValue != nil {
		return RuntimeArtifactManifest{}, errorValue
	}
	return RuntimeArtifactManifest{
		GuestInitSHA256:     guestInitSHA256,
		PrepareScriptSHA256: prepareScriptSHA256,
		BaseSourceSHA256:    baseSourceSHA256,
	}, nil
}

func calculateRuntimeBaseSourceSHA256(repositoryRootPath string) (string, error) {
	sourceRootPath := filepath.Join(repositoryRootPath, BlueclawSubmodulePath)
	return calculateSelectedPathsSHA256(sourceRootPath, []string{
		"go.mod",
		"go.sum",
		"cmd/blueclaw-guest-healthd",
		"cmd/blueclaw-vsock-http-proxy",
		"cmd/blueclaw-posix-helper",
		"internal/policy",
		"internal/security/posix_identity.go",
		"tools/graphiti_memoryd",
	})
}

func RuntimeArtifactFilePath(artifactDirectoryPath string, manifest RuntimeArtifactManifest, fileName string) (string, error) {
	for _, manifestFile := range manifest.Files {
		if manifestFile.Name == fileName {
			return filepath.Join(artifactDirectoryPath, filepath.Clean(manifestFile.Path)), nil
		}
	}

	return "", fmt.Errorf("Blueclaw runtime artifact %q was not found in manifest", fileName)
}

func FindRuntimeArtifactManifestFile(manifest RuntimeArtifactManifest, fileName string) (RuntimeArtifactManifestFile, error) {
	for _, manifestFile := range manifest.Files {
		if manifestFile.Name == fileName {
			return manifestFile, nil
		}
	}
	return RuntimeArtifactManifestFile{}, fmt.Errorf("Blueclaw runtime artifact %q was not found in manifest", fileName)
}

func validateRuntimeArtifactFile(artifactDirectoryPath string, manifestFile RuntimeArtifactManifestFile) error {
	if strings.TrimSpace(manifestFile.Path) == "" {
		return fmt.Errorf("Blueclaw runtime artifact %q has an empty path", manifestFile.Name)
	}
	if filepath.IsAbs(manifestFile.Path) || strings.HasPrefix(filepath.Clean(manifestFile.Path), "..") {
		return fmt.Errorf("Blueclaw runtime artifact %q has unsafe path %q", manifestFile.Name, manifestFile.Path)
	}

	artifactPath := filepath.Join(artifactDirectoryPath, filepath.Clean(manifestFile.Path))
	fileInformation, errorValue := os.Stat(artifactPath)
	if errorValue != nil {
		return fmt.Errorf("stat Blueclaw runtime artifact %q: %w", manifestFile.Name, errorValue)
	}
	if fileInformation.IsDir() || fileInformation.Size() == 0 {
		return fmt.Errorf("Blueclaw runtime artifact %q is empty or not a regular file", manifestFile.Name)
	}

	actualSHA256, errorValue := calculateFileSHA256(artifactPath)
	if errorValue != nil {
		return fmt.Errorf("hash Blueclaw runtime artifact %q: %w", manifestFile.Name, errorValue)
	}
	if !strings.EqualFold(actualSHA256, strings.TrimSpace(manifestFile.SHA256)) {
		return fmt.Errorf("Blueclaw runtime artifact %q checksum mismatch", manifestFile.Name)
	}

	return nil
}

func calculateFileSHA256(filePath string) (string, error) {
	file, errorValue := os.Open(filePath)
	if errorValue != nil {
		return "", errorValue
	}
	defer file.Close()

	hash := sha256.New()
	if _, errorValue := io.Copy(hash, file); errorValue != nil {
		return "", errorValue
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func gitRevision(repositoryPath string) (string, error) {
	output, errorValue := exec.Command("git", "-C", repositoryPath, "rev-parse", "HEAD").Output()
	if errorValue != nil {
		return "", errorValue
	}
	revision := strings.TrimSpace(string(output))
	if revision == "" {
		return "", fmt.Errorf("empty git revision for %s", repositoryPath)
	}
	if suffix := workingTreeDirtySuffix(repositoryPath); suffix != "" {
		revision = revision + "-" + suffix
	}
	return revision, nil
}

// A dirty working tree gets a content-derived suffix so a locally-built payload
// (INTERNKIM_BLUECLAW_USE_LOCAL) deploys as a distinct release instead of colliding
// with the committed HEAD SHA, which the OTA engine would skip as already deployed.
// The suffix changes only when the working-tree content changes, so unchanged rebuilds
// stay idempotent.
func workingTreeDirtySuffix(repositoryPath string) string {
	trackedDiff, errorValue := exec.Command("git", "-C", repositoryPath, "diff", "HEAD").Output()
	if errorValue != nil {
		return ""
	}
	untrackedList, errorValue := exec.Command("git", "-C", repositoryPath, "ls-files", "--others", "--exclude-standard").Output()
	if errorValue != nil {
		return ""
	}
	if len(strings.TrimSpace(string(trackedDiff))) == 0 && len(strings.TrimSpace(string(untrackedList))) == 0 {
		return ""
	}
	hash := sha256.Sum256(append(trackedDiff, untrackedList...))
	return "dirty-" + hex.EncodeToString(hash[:])[:12]
}
