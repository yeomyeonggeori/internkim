package admind

import (
	"os"
	"path/filepath"
)

func writeFileAtomically(path string, payload []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	temporary, errorValue := os.CreateTemp(directory, filepath.Base(path)+".tmp-*")
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := temporary.Name()
	cleanup := func() { _ = os.Remove(temporaryPath) }
	if _, errorValue := temporary.Write(payload); errorValue != nil {
		_ = temporary.Close()
		cleanup()
		return errorValue
	}
	if errorValue := temporary.Chmod(mode); errorValue != nil {
		_ = temporary.Close()
		cleanup()
		return errorValue
	}
	if errorValue := temporary.Sync(); errorValue != nil {
		_ = temporary.Close()
		cleanup()
		return errorValue
	}
	if errorValue := temporary.Close(); errorValue != nil {
		cleanup()
		return errorValue
	}
	if errorValue := os.Rename(temporaryPath, path); errorValue != nil {
		cleanup()
		return errorValue
	}
	return syncParentDirectory(path)
}

func syncParentDirectory(path string) error {
	directory, errorValue := os.Open(filepath.Dir(path))
	if errorValue != nil {
		return errorValue
	}
	defer directory.Close()
	return directory.Sync()
}
