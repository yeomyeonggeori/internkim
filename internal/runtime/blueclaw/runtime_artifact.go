package blueclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type RuntimeArtifactManifest struct {
	RuntimeName string                        `json:"runtimeName"`
	Platform    string                        `json:"platform"`
	Version     string                        `json:"version"`
	Files       []RuntimeArtifactManifestFile `json:"files"`
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

func RuntimeArtifactFilePath(artifactDirectoryPath string, manifest RuntimeArtifactManifest, fileName string) (string, error) {
	for _, manifestFile := range manifest.Files {
		if manifestFile.Name == fileName {
			return filepath.Join(artifactDirectoryPath, filepath.Clean(manifestFile.Path)), nil
		}
	}

	return "", fmt.Errorf("Blueclaw runtime artifact %q was not found in manifest", fileName)
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
