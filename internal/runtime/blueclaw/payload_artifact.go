package blueclaw

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PayloadArtifactManifest struct {
	RuntimeName      string `json:"runtimeName"`
	Platform         string `json:"platform"`
	BlueclawRevision string `json:"blueclawRevision"`
	BlueclawSHA256   string `json:"blueclawSHA256"`
	MigrationsSHA256 string `json:"migrationsSHA256"`
}

func ValidatePayloadArtifactDirectory(artifactDirectoryPath string) (PayloadArtifactManifest, error) {
	manifestPath := filepath.Join(artifactDirectoryPath, "manifest.json")
	manifestDocument, errorValue := os.ReadFile(manifestPath)
	if errorValue != nil {
		return PayloadArtifactManifest{}, fmt.Errorf("read Blueclaw payload manifest: %w", errorValue)
	}

	manifest, errorValue := ParsePayloadArtifactManifest(manifestDocument)
	if errorValue != nil {
		return PayloadArtifactManifest{}, errorValue
	}
	return validatePayloadArtifactDirectory(artifactDirectoryPath, manifest)
}

func ParsePayloadArtifactManifest(manifestDocument []byte) (PayloadArtifactManifest, error) {
	var manifest PayloadArtifactManifest
	if errorValue := json.Unmarshal(manifestDocument, &manifest); errorValue != nil {
		return PayloadArtifactManifest{}, fmt.Errorf("parse Blueclaw payload manifest: %w", errorValue)
	}
	if strings.TrimSpace(manifest.RuntimeName) != "internkim-blueclaw-payload" {
		return PayloadArtifactManifest{}, fmt.Errorf("unexpected Blueclaw payload name %q", manifest.RuntimeName)
	}
	if strings.TrimSpace(manifest.Platform) != "linux-arm64" {
		return PayloadArtifactManifest{}, fmt.Errorf("unexpected Blueclaw payload platform %q", manifest.Platform)
	}
	return manifest, nil
}

func ValidatePayloadArtifactSource(repositoryRootPath string, manifest PayloadArtifactManifest) error {
	expectedManifest, errorValue := ExpectedPayloadArtifactSource(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(manifest.BlueclawRevision) == "" {
		return fmt.Errorf("Blueclaw payload source metadata is missing; run `make prepare-blueclaw-payload`")
	}
	if manifest.BlueclawRevision != expectedManifest.BlueclawRevision {
		return fmt.Errorf("Blueclaw payload was built from Blueclaw %s, current source is %s; run `make prepare-blueclaw-payload`", manifest.BlueclawRevision, expectedManifest.BlueclawRevision)
	}
	if manifest.MigrationsSHA256 != expectedManifest.MigrationsSHA256 {
		return fmt.Errorf("Blueclaw payload migrations are stale; run `make prepare-blueclaw-payload`")
	}
	return nil
}

func ExpectedPayloadArtifactSource(repositoryRootPath string) (PayloadArtifactManifest, error) {
	blueclawRevision, errorValue := gitRevision(filepath.Join(repositoryRootPath, BlueclawSubmodulePath))
	if errorValue != nil {
		return PayloadArtifactManifest{}, fmt.Errorf("resolve Blueclaw revision: %w", errorValue)
	}
	migrationsSHA256, errorValue := calculateDirectorySHA256(filepath.Join(repositoryRootPath, BlueclawSubmodulePath, "migrations"))
	if errorValue != nil {
		return PayloadArtifactManifest{}, fmt.Errorf("hash Blueclaw migrations: %w", errorValue)
	}
	return PayloadArtifactManifest{
		RuntimeName:      "internkim-blueclaw-payload",
		Platform:         "linux-arm64",
		BlueclawRevision: blueclawRevision,
		MigrationsSHA256: migrationsSHA256,
	}, nil
}

func PayloadWorkspacePath(artifactDirectoryPath string) string {
	return filepath.Join(artifactDirectoryPath, "workspace")
}

func validatePayloadArtifactDirectory(artifactDirectoryPath string, manifest PayloadArtifactManifest) (PayloadArtifactManifest, error) {
	blueclawBinaryPath := filepath.Join(artifactDirectoryPath, "workspace", ".blueclaw", "runtime", "current", "bin", "blueclaw")
	blueclawSHA256, errorValue := calculateFileSHA256(blueclawBinaryPath)
	if errorValue != nil {
		return PayloadArtifactManifest{}, fmt.Errorf("hash Blueclaw payload binary: %w", errorValue)
	}
	if !strings.EqualFold(blueclawSHA256, strings.TrimSpace(manifest.BlueclawSHA256)) {
		return PayloadArtifactManifest{}, fmt.Errorf("Blueclaw payload binary checksum mismatch")
	}

	migrationsSHA256, errorValue := calculateDirectorySHA256(filepath.Join(artifactDirectoryPath, "workspace", ".blueclaw", "runtime", "current", "migrations"))
	if errorValue != nil {
		return PayloadArtifactManifest{}, fmt.Errorf("hash Blueclaw payload migrations: %w", errorValue)
	}
	if !strings.EqualFold(migrationsSHA256, strings.TrimSpace(manifest.MigrationsSHA256)) {
		return PayloadArtifactManifest{}, fmt.Errorf("Blueclaw payload migrations checksum mismatch")
	}

	return manifest, nil
}

func calculateDirectorySHA256(directoryPath string) (string, error) {
	var filePaths []string
	errorValue := filepath.WalkDir(directoryPath, func(filePath string, directoryEntry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if directoryEntry.IsDir() {
			return nil
		}
		if isAppleMetadataPath(filePath) {
			return nil
		}
		filePaths = append(filePaths, filePath)
		return nil
	})
	if errorValue != nil {
		return "", errorValue
	}
	sort.Strings(filePaths)

	hash := sha256.New()
	for _, filePath := range filePaths {
		relativePath, errorValue := filepath.Rel(directoryPath, filePath)
		if errorValue != nil {
			return "", errorValue
		}
		io.WriteString(hash, filepath.ToSlash(relativePath))
		io.WriteString(hash, "\n")
		file, errorValue := os.Open(filePath)
		if errorValue != nil {
			return "", errorValue
		}
		_, copyError := io.Copy(hash, file)
		closeError := file.Close()
		if copyError != nil {
			return "", copyError
		}
		if closeError != nil {
			return "", closeError
		}
		io.WriteString(hash, "\n")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func isAppleMetadataPath(path string) bool {
	baseName := filepath.Base(path)
	return baseName == ".DS_Store" || strings.HasPrefix(baseName, "._")
}

func calculateSelectedPathsSHA256(rootPath string, relativePaths []string) (string, error) {
	hash := sha256.New()
	for _, relativePath := range relativePaths {
		cleanRelativePath := filepath.Clean(relativePath)
		fullPath := filepath.Join(rootPath, cleanRelativePath)
		fileInformation, errorValue := os.Stat(fullPath)
		if errorValue != nil {
			return "", errorValue
		}
		io.WriteString(hash, filepath.ToSlash(cleanRelativePath))
		io.WriteString(hash, "\n")
		if fileInformation.IsDir() {
			directorySHA256, errorValue := calculateDirectorySHA256(fullPath)
			if errorValue != nil {
				return "", errorValue
			}
			io.WriteString(hash, directorySHA256)
		} else {
			fileSHA256, errorValue := calculateFileSHA256(fullPath)
			if errorValue != nil {
				return "", errorValue
			}
			io.WriteString(hash, fileSHA256)
		}
		io.WriteString(hash, "\n")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
