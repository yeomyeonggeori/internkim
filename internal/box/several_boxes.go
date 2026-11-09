package box

import (
	"errors"
	"io/fs"
	"os"
)

func NoteSeveralBoxes(markerPath string, hasSeveralBoxes bool) error {
	if hasSeveralBoxes {
		return os.WriteFile(markerPath, nil, 0o600)
	}
	if errorValue := os.Remove(markerPath); errorValue != nil && !errors.Is(errorValue, fs.ErrNotExist) {
		return errorValue
	}
	return nil
}

func RunsOnSeveralBoxes(markerPath string) bool {
	_, errorValue := os.Stat(markerPath)
	return errorValue == nil
}
