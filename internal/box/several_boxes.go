package box

import (
	"errors"
	"io/fs"
	"os"
	"strings"
)

const (
	severalBoxesWord = "several"
	oneBoxWord       = "one"
)

func NoteSeveralBoxes(markerPath string, hasSeveralBoxes bool) error {
	word := oneBoxWord
	if hasSeveralBoxes {
		word = severalBoxesWord
	}
	partialPath := markerPath + ".partial"
	if errorValue := os.WriteFile(partialPath, []byte(word+"\n"), 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(partialPath, markerPath)
}

func CompanyBoxCount(markerPath string) (hasSeveralBoxes bool, isKnown bool) {
	contents, errorValue := os.ReadFile(markerPath)
	if errors.Is(errorValue, fs.ErrNotExist) || errorValue != nil {
		return false, false
	}
	switch strings.TrimSpace(string(contents)) {
	case severalBoxesWord:
		return true, true
	case oneBoxWord:
		return false, true
	}
	return false, false
}
