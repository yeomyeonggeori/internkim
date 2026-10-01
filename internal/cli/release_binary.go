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

type releaseTarget struct {
	OperatingSystem string
	Architecture    string
}

var binaryReleaseTargets = []releaseTarget{
	{OperatingSystem: "darwin", Architecture: "arm64"},
	{OperatingSystem: "darwin", Architecture: "amd64"},
	{OperatingSystem: "linux", Architecture: "arm64"},
	{OperatingSystem: "linux", Architecture: "amd64"},
}

type releaseProduct struct {
	Name        string
	BinaryName  string
	PackagePath string
}

var companionProduct = releaseProduct{
	Name:        "companion",
	BinaryName:  companionBinaryName,
	PackagePath: "./cmd/internkim-companion",
}

const releaseChecksumsName = "SHA256SUMS"

type binaryBuilder func(target releaseTarget, outputPath string) error

func (target releaseTarget) String() string {
	return target.OperatingSystem + "/" + target.Architecture
}

func (product releaseProduct) ArtifactName(target releaseTarget) string {
	return product.BinaryName + "-" + target.OperatingSystem + "-" + target.Architecture
}

func (product releaseProduct) Prefixes(releaseID string) []string {
	return []string{product.Name + "/" + releaseID + "/", product.LatestPrefix()}
}

func (product releaseProduct) LatestPrefix() string {
	return product.Name + "/latest/"
}

func publishBinaryRelease(product releaseProduct, releaseID string, publisher releaseObjectPublisher, build binaryBuilder, output io.Writer) error {
	temporaryDirectoryPath, errorValue := os.MkdirTemp("", "internkim-"+product.Name+"-release-*")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(temporaryDirectoryPath)
	binaries, checksums, errorValue := buildEveryTarget(product, build, temporaryDirectoryPath, output)
	if errorValue != nil {
		return errorValue
	}
	for _, prefix := range product.Prefixes(releaseID) {
		for artifactName, binary := range binaries {
			if errorValue := publisher.PutObject(prefix+artifactName, binary, "application/octet-stream"); errorValue != nil {
				return errorValue
			}
		}
		if errorValue := publisher.PutObject(prefix+releaseChecksumsName, []byte(checksums), "text/plain"); errorValue != nil {
			return errorValue
		}
	}
	fmt.Fprintf(output, "published %s %s -> %s\n", product.Name, releaseID, publisher.PublicURL(product.LatestPrefix()))
	return nil
}

func buildEveryTarget(product releaseProduct, build binaryBuilder, directoryPath string, output io.Writer) (map[string][]byte, string, error) {
	binaries := map[string][]byte{}
	var checksums strings.Builder
	for _, target := range binaryReleaseTargets {
		artifactName := product.ArtifactName(target)
		outputPath := filepath.Join(directoryPath, artifactName)
		if errorValue := build(target, outputPath); errorValue != nil {
			return nil, "", errorValue
		}
		binary, errorValue := os.ReadFile(outputPath)
		if errorValue != nil {
			return nil, "", errorValue
		}
		digest := sha256.Sum256(binary)
		checksum := hex.EncodeToString(digest[:])
		binaries[artifactName] = binary
		checksums.WriteString(checksum + "  " + artifactName + "\n")
		fmt.Fprintf(output, "built %s %s\n", artifactName, checksum[:12])
	}
	return binaries, checksums.String(), nil
}

func crossCompileProduct(repositoryRootPath string, product releaseProduct, linkerFlags string) binaryBuilder {
	return func(target releaseTarget, outputPath string) error {
		command := exec.Command("go", "build", "-trimpath", "-ldflags", linkerFlags, "-o", outputPath, product.PackagePath)
		command.Dir = repositoryRootPath
		command.Env = append(os.Environ(), releaseBuildEnvironment(target)...)
		output, errorValue := command.CombinedOutput()
		if errorValue != nil {
			return fmt.Errorf("build %s for %s: %s", product.Name, target, strings.TrimSpace(string(output)))
		}
		return nil
	}
}

func releaseBuildEnvironment(target releaseTarget) []string {
	return []string{
		"GOOS=" + target.OperatingSystem,
		"GOARCH=" + target.Architecture,
		"CGO_ENABLED=" + releaseCgoSetting(target),
	}
}

func releaseCgoSetting(target releaseTarget) string {
	if target.OperatingSystem == "darwin" {
		return "1"
	}
	return "0"
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
	build := crossCompileProduct(repositoryRootPath, companionProduct, "-s -w")
	return publishBinaryRelease(companionProduct, releaseID, publisher, build, os.Stdout)
}
