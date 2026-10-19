package companyhost

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const boxIdentityFileName = "identity.json"

func keepingThisBoxThrough(boxStatePath string, restore func() error) error {
	asidePath := filepath.Join(filepath.Dir(filepath.Dir(boxStatePath)), ".internkim-box-during-restore")
	isHeld, errorValue := setThisBoxAside(boxStatePath, asidePath)
	if errorValue != nil {
		return errorValue
	}
	restoreError := restore()
	if !isHeld {
		return restoreError
	}
	return errors.Join(restoreError, putThisBoxBack(boxStatePath, asidePath))
}

func setThisBoxAside(boxStatePath, asidePath string) (bool, error) {
	if _, errorValue := os.Stat(filepath.Join(asidePath, boxIdentityFileName)); errorValue == nil {
		return true, nil
	}
	if _, errorValue := os.Stat(filepath.Join(boxStatePath, boxIdentityFileName)); errors.Is(errorValue, fs.ErrNotExist) {
		return false, nil
	} else if errorValue != nil {
		return false, errorValue
	}
	if errorValue := os.RemoveAll(asidePath); errorValue != nil {
		return false, errorValue
	}
	return true, os.Rename(boxStatePath, asidePath)
}

func putThisBoxBack(boxStatePath, asidePath string) error {
	if errorValue := os.RemoveAll(boxStatePath); errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(boxStatePath), 0o755); errorValue != nil {
		return errorValue
	}
	return os.Rename(asidePath, boxStatePath)
}
