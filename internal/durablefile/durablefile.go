package durablefile

import (
	"errors"
	"os"
	"path/filepath"
)

// Write leaves path holding either what it held before or all of document,
// even when the computer loses power partway, because a file a crash leaves
// empty reads as a broken install on every boot after it.
func Write(path string, document []byte, mode os.FileMode) error {
	temporary, errorValue := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if errorValue != nil {
		return errorValue
	}
	defer os.Remove(temporary.Name())
	if errorValue := fill(temporary, document, mode); errorValue != nil {
		temporary.Close()
		return errorValue
	}
	if errorValue := temporary.Close(); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Rename(temporary.Name(), path); errorValue != nil {
		return errorValue
	}
	return syncDirectory(filepath.Dir(path))
}

// ReplaceLink points the symbolic link at path to target in one rename, so no
// moment exists in which the link is missing.
func ReplaceLink(path, target string) error {
	temporaryPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".linking")
	if errorValue := os.Remove(temporaryPath); errorValue != nil && !errors.Is(errorValue, os.ErrNotExist) {
		return errorValue
	}
	if errorValue := os.Symlink(target, temporaryPath); errorValue != nil {
		return errorValue
	}
	if errorValue := os.Rename(temporaryPath, path); errorValue != nil {
		return errors.Join(errorValue, os.Remove(temporaryPath))
	}
	return syncDirectory(filepath.Dir(path))
}

func fill(file *os.File, document []byte, mode os.FileMode) error {
	if errorValue := file.Chmod(mode); errorValue != nil {
		return errorValue
	}
	if _, errorValue := file.Write(document); errorValue != nil {
		return errorValue
	}
	return file.Sync()
}

func syncDirectory(path string) error {
	directory, errorValue := os.Open(path)
	if errorValue != nil {
		return errorValue
	}
	defer directory.Close()
	return directory.Sync()
}
