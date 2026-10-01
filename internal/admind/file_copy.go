package admind

import (
	"io"
	"os"
	"path/filepath"
)

func copyDirectory(sourceRoot string, targetRoot string) error {
	return filepath.Walk(sourceRoot, func(sourcePath string, information os.FileInfo, walkError error) error {
		if walkError != nil {
			return walkError
		}
		relativePath, errorValue := filepath.Rel(sourceRoot, sourcePath)
		if errorValue != nil || relativePath == "." {
			return errorValue
		}
		targetPath := filepath.Join(targetRoot, relativePath)
		if information.IsDir() {
			return os.MkdirAll(targetPath, information.Mode())
		}
		if !information.Mode().IsRegular() {
			return nil
		}
		if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
			return errorValue
		}
		sourceFile, errorValue := os.Open(sourcePath)
		if errorValue != nil {
			return errorValue
		}
		defer sourceFile.Close()
		targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, information.Mode())
		if errorValue != nil {
			return errorValue
		}
		_, copyErrorValue := io.Copy(targetFile, sourceFile)
		closeErrorValue := targetFile.Close()
		if copyErrorValue != nil {
			return copyErrorValue
		}
		return closeErrorValue
	})
}

func copyRegularFile(sourcePath string, targetPath string) error {
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()
	sourceInfo, errorValue := sourceFile.Stat()
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o755); errorValue != nil {
		return errorValue
	}
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode())
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(targetFile, sourceFile)
	closeErrorValue := targetFile.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}

func copyFile(sourcePath string, targetPath string, mode os.FileMode) error {
	sourceFile, errorValue := os.Open(sourcePath)
	if errorValue != nil {
		return errorValue
	}
	defer sourceFile.Close()
	if errorValue := os.MkdirAll(filepath.Dir(targetPath), 0o700); errorValue != nil {
		return errorValue
	}
	targetFile, errorValue := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if errorValue != nil {
		return errorValue
	}
	_, copyErrorValue := io.Copy(targetFile, sourceFile)
	closeErrorValue := targetFile.Close()
	if copyErrorValue != nil {
		return copyErrorValue
	}
	return closeErrorValue
}
