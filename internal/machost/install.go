package machost

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const virtualizationEntitlement = "com.apple.security.virtualization"

type CodesignRunner func(executablePath string) (string, error)

func RequireVirtualMachineMonitor(executablePath string, runCodesign CodesignRunner) error {
	if _, errorValue := os.Stat(executablePath); errorValue != nil {
		return fmt.Errorf("no vfkit at %s; install it with `brew install vfkit` or from the crc-org/vfkit releases", executablePath)
	}
	entitlements, errorValue := runCodesign(executablePath)
	if errorValue != nil {
		return fmt.Errorf("read the entitlements of %s: %w", executablePath, errorValue)
	}
	if !strings.Contains(entitlements, virtualizationEntitlement) {
		return fmt.Errorf("%s carries no %s entitlement, so Virtualization.framework will refuse it; install an official signed release", executablePath, virtualizationEntitlement)
	}
	return nil
}

func ReadCodesignEntitlements(executablePath string) (string, error) {
	output, errorValue := exec.Command("codesign", "-d", "--entitlements", "-", executablePath).CombinedOutput()
	if errorValue != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), errorValue)
	}
	return string(output), nil
}

func FindVirtualMachineMonitor(layout Layout) string {
	bundledPath := filepath.Join(layout.InstallRootPath, "bin", "vfkit")
	if _, errorValue := os.Stat(bundledPath); errorValue == nil {
		return bundledPath
	}
	if resolvedPath, errorValue := exec.LookPath("vfkit"); errorValue == nil {
		return resolvedPath
	}
	return bundledPath
}

func InstallRuntimeArtifacts(layout Layout, artifactDirectoryPath string) error {
	for _, directoryPath := range []string{layout.InstallRootPath, layout.LogDirectoryPath(), filepath.Dir(layout.SupervisorBinaryPath())} {
		if errorValue := os.MkdirAll(directoryPath, deliveryDirectoryMode); errorValue != nil {
			return errorValue
		}
	}
	for artifactName, destinationPath := range map[string]string{
		"vmlinux.bin": layout.KernelImagePath(),
		"rootfs.ext4": layout.RootFilesystemImagePath(),
	} {
		if errorValue := copyArtifactSparsely(filepath.Join(artifactDirectoryPath, artifactName), destinationPath); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

// APFS clones the file rather than reading it, which matters because the root filesystem is
// four gigabytes and every install would otherwise copy it byte by byte.
func copyArtifactSparsely(sourcePath string, destinationPath string) error {
	if _, errorValue := exec.Command("cp", "-c", sourcePath, destinationPath).CombinedOutput(); errorValue == nil {
		return nil
	}
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()

	destinationFile, errorValue := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, deliveryFileMode)
	if errorValue != nil {
		return errorValue
	}
	defer destinationFile.Close()

	_, errorValue = io.Copy(destinationFile, sourceFile)
	return errorValue
}
