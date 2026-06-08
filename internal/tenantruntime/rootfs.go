package tenantruntime

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func (service Service) CreateTenantFromTemplate(manifest Manifest, templateRootFilesystemPath string) (TenantStatus, error) {
	status, errorValue := service.CreateTenant(manifest)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if strings.TrimSpace(templateRootFilesystemPath) == "" {
		return status, nil
	}
	paths, errorValue := BuildRuntimePaths(service.BasePath, manifest.TenantID)
	if errorValue != nil {
		return TenantStatus{}, errorValue
	}
	if errorValue := copyRootFilesystem(templateRootFilesystemPath, paths.ContainerRootPath); errorValue != nil {
		return TenantStatus{}, errorValue
	}
	return service.Status(manifest.TenantID)
}

func bootableRootFilesystemExists(rootFilesystemPath string) bool {
	for _, initPath := range []string{
		"usr/lib/systemd/systemd",
		"lib/systemd/systemd",
		"sbin/init",
	} {
		if fileExists(filepath.Join(rootFilesystemPath, initPath)) {
			return true
		}
	}
	return false
}

func copyRootFilesystem(sourcePath string, destinationPath string) error {
	if strings.TrimSpace(sourcePath) == "" {
		return errors.New("template rootfs path is required")
	}
	sourceRootPath := filepath.Clean(sourcePath)
	destinationRootPath := filepath.Clean(destinationPath)
	if !directoryExists(sourceRootPath) {
		return errors.New("template rootfs directory does not exist")
	}
	return filepath.WalkDir(sourceRootPath, func(path string, directoryEntry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceRootPath, path)
		if errorValue != nil {
			return errorValue
		}
		if relativePath == "." {
			return nil
		}
		targetPath := filepath.Join(destinationRootPath, relativePath)
		info, errorValue := directoryEntry.Info()
		if errorValue != nil {
			return errorValue
		}
		if directoryEntry.Type()&os.ModeSymlink != 0 {
			return copySymlink(path, targetPath)
		}
		if directoryEntry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		return copyRegularFile(path, targetPath, info.Mode().Perm())
	})
}

func copySymlink(sourcePath string, destinationPath string) error {
	linkTarget, errorValue := os.Readlink(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(destinationPath), 0o755); errorValue != nil {
		return errorValue
	}
	if fileExists(destinationPath) {
		if errorValue := os.Remove(destinationPath); errorValue != nil {
			return errorValue
		}
	}
	return os.Symlink(linkTarget, destinationPath)
}

func copyRegularFile(sourcePath string, destinationPath string, mode os.FileMode) error {
	if errorValue := os.MkdirAll(filepath.Dir(destinationPath), 0o755); errorValue != nil {
		return errorValue
	}
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()
	destinationFile, errorValue := os.OpenFile(destinationPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if errorValue != nil {
		return errorValue
	}
	defer destinationFile.Close()
	_, errorValue = io.Copy(destinationFile, sourceFile)
	return errorValue
}
