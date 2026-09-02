package machost

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	deliveryDirectoryMode  fs.FileMode = 0o755
	deliveryFileMode       fs.FileMode = 0o644
	deliveryExecutableMode fs.FileMode = 0o755
)

type DeliverySources struct {
	PayloadRuntimePath       string
	SkillPaths               []string
	RuntimeConfigurationJSON string
	PolicyJSON               string
}

func WriteDeliveryDirectory(layout Layout, sources DeliverySources) error {
	if errorValue := UnsealDeliveryDirectory(layout); errorValue != nil {
		return errorValue
	}
	if errorValue := replaceDeliveryTree(sources.PayloadRuntimePath, layout.DeliveryRuntimePath()); errorValue != nil {
		return errorValue
	}
	if errorValue := os.RemoveAll(layout.DeliverySkillsPath()); errorValue != nil {
		return errorValue
	}
	rootBySkillName := map[string]string{}
	for _, skillPath := range sources.SkillPaths {
		if errorValue := recordDeliveredSkillNames(skillPath, rootBySkillName); errorValue != nil {
			return errorValue
		}
		if errorValue := copyDeliveryTree(skillPath, layout.DeliverySkillsPath()); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := os.MkdirAll(layout.DeliveryConfigurationPath(), deliveryDirectoryMode); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(layout.RuntimeConfigurationPath(), []byte(sources.RuntimeConfigurationJSON), deliveryFileMode); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(layout.PolicyPath(), []byte(sources.PolicyJSON), deliveryFileMode)
}

func recordDeliveredSkillNames(skillPath string, rootBySkillName map[string]string) error {
	entries, errorValue := os.ReadDir(skillPath)
	if errorValue != nil {
		if os.IsNotExist(errorValue) {
			return nil
		}
		return errorValue
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if previousRoot, isDuplicate := rootBySkillName[entry.Name()]; isDuplicate {
			return fmt.Errorf("skill %q is provided by both %s and %s", entry.Name(), previousRoot, skillPath)
		}
		rootBySkillName[entry.Name()] = skillPath
	}
	return nil
}

func replaceDeliveryTree(sourcePath string, destinationPath string) error {
	if errorValue := os.RemoveAll(destinationPath); errorValue != nil {
		return errorValue
	}
	return copyDeliveryTree(sourcePath, destinationPath)
}

// virtio-fs passes uid and gid through untranslated, so the guest's blueclaw sees the host's
// numbers and no group can span the boundary. The mode is what carries access, which is why
// these are the same modes the Linux delivery refresh sets.
func copyDeliveryTree(sourcePath string, destinationPath string) error {
	return filepath.WalkDir(sourcePath, func(currentPath string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourcePath, currentPath)
		if errorValue != nil {
			return errorValue
		}
		targetPath := filepath.Join(destinationPath, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, deliveryDirectoryMode)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		entryInformation, errorValue := entry.Info()
		if errorValue != nil {
			return errorValue
		}
		return copyDeliveryFile(currentPath, targetPath, deliveryFileModeFor(relativePath, entryInformation.Mode()))
	})
}

func deliveryFileModeFor(relativePath string, sourceMode fs.FileMode) fs.FileMode {
	if strings.HasPrefix(relativePath, "bin"+string(filepath.Separator)) || sourceMode.Perm()&0o100 != 0 {
		return deliveryExecutableMode
	}
	return deliveryFileMode
}

func copyDeliveryFile(sourcePath string, destinationPath string, mode fs.FileMode) error {
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()

	if errorValue := os.MkdirAll(filepath.Dir(destinationPath), deliveryDirectoryMode); errorValue != nil {
		return errorValue
	}
	destinationFile, errorValue := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if errorValue != nil {
		return errorValue
	}
	defer destinationFile.Close()

	if _, errorValue := io.Copy(destinationFile, sourceFile); errorValue != nil {
		return errorValue
	}
	return os.Chmod(destinationPath, mode)
}
