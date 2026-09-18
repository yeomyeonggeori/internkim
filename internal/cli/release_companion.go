package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type companionReleaseTarget struct {
	OperatingSystem string
	Architecture    string
}

var companionReleaseTargets = []companionReleaseTarget{
	{OperatingSystem: "darwin", Architecture: "arm64"},
	{OperatingSystem: "darwin", Architecture: "amd64"},
	{OperatingSystem: "linux", Architecture: "arm64"},
	{OperatingSystem: "linux", Architecture: "amd64"},
}

const (
	companionReleasePrefix       = "companion/"
	companionLatestReleasePrefix = companionReleasePrefix + "latest/"
	companionChecksumsName       = "SHA256SUMS"
)

type companionBinaryBuilder func(target companionReleaseTarget, outputPath string) error

func (target companionReleaseTarget) String() string {
	return target.OperatingSystem + "/" + target.Architecture
}

func (target companionReleaseTarget) BinaryName() string {
	return companionBinaryName + "-" + target.OperatingSystem + "-" + target.Architecture
}

func runReleaseCompanion(arguments []string) error {
	repositoryRootPath, errorValue := resolveRepositoryRootPath()
	if errorValue != nil {
		return errorValue
	}
	releaseID := firstNonEmptyString(commandArgumentValue(arguments, "--release", ""), defaultReleaseID(repositoryRootPath))
	publisher, errorValue := releasePublisherFromEnvironment(repositoryRootPath)
	if errorValue != nil {
		return errorValue
	}
	return publishCompanionRelease(releaseID, publisher, buildCompanionReleaseBinary(repositoryRootPath), os.Stdout)
}

func publishCompanionRelease(releaseID string, publisher releaseObjectPublisher, build companionBinaryBuilder, output io.Writer) error {
	temporaryDirectoryPath, errorValue := os.MkdirTemp("", "internkim-companion-release-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(temporaryDirectoryPath)
	prefixes := companionReleasePrefixes(releaseID)
	var checksums strings.Builder
	for _, target := range companionReleaseTargets {
		binary, errorValue := buildCompanionBinary(build, target, temporaryDirectoryPath)
		if errorValue != nil {
			return errorValue
		}
		digest := sha256.Sum256(binary)
		checksum := hex.EncodeToString(digest[:])
		for _, prefix := range prefixes {
			if errorValue := publisher.PutObject(prefix+target.BinaryName(), binary, "application/octet-stream"); errorValue != nil {
				return errorValue
			}
		}
		fmt.Fprintf(output, "uploaded %s %s\n", target.BinaryName(), checksum[:12])
		checksums.WriteString(checksum + "  " + target.BinaryName() + "\n")
	}
	for _, prefix := range prefixes {
		if errorValue := publisher.PutObject(prefix+companionChecksumsName, []byte(checksums.String()), "text/plain"); errorValue != nil {
			return errorValue
		}
	}
	fmt.Fprintf(output, "published companion %s -> %s\n", releaseID, publisher.PublicURL(companionLatestReleasePrefix))
	return nil
}

func companionReleasePrefixes(releaseID string) []string {
	return []string{companionReleasePrefix + releaseID + "/", companionLatestReleasePrefix}
}

func buildCompanionBinary(build companionBinaryBuilder, target companionReleaseTarget, directoryPath string) ([]byte, error) {
	outputPath := filepath.Join(directoryPath, target.BinaryName())
	if errorValue := build(target, outputPath); errorValue != nil {
		return nil, errorValue
	}
	return os.ReadFile(outputPath)
}

func buildCompanionReleaseBinary(repositoryRootPath string) companionBinaryBuilder {
	return func(target companionReleaseTarget, outputPath string) error {
		command := exec.Command("go", "build", "-trimpath", "-ldflags", "-s -w", "-o", outputPath, "./cmd/internkim-companion")
		command.Dir = repositoryRootPath
		command.Env = append(os.Environ(), companionBuildEnvironment(target)...)
		output, errorValue := command.CombinedOutput()
		if errorValue != nil {
			return fmt.Errorf("build companion for %s: %s", target, strings.TrimSpace(string(output)))
		}
		return nil
	}
}

func companionBuildEnvironment(target companionReleaseTarget) []string {
	return []string{
		"GOOS=" + target.OperatingSystem,
		"GOARCH=" + target.Architecture,
		"CGO_ENABLED=" + companionCgoSetting(target),
	}
}

func companionCgoSetting(target companionReleaseTarget) string {
	if target.OperatingSystem == "darwin" {
		return "1"
	}
	return "0"
}
