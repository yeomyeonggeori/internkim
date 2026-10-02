package boxwifi

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	DefaultMadeOnPath = "/boot/firmware/internkim-made-on"
	madeOnLayout      = "2006-01-02"
)

func ReadMadeOn(path string) (time.Time, error) {
	document, errorValue := os.ReadFile(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if errorValue != nil {
		return time.Time{}, fmt.Errorf("reading the day this box was made from %s: %w", path, errorValue)
	}
	madeOn, errorValue := time.Parse(madeOnLayout, strings.TrimSpace(string(document)))
	if errorValue != nil {
		return time.Time{}, fmt.Errorf("%s should hold the day this box was made as YYYY-MM-DD: %w", path, errorValue)
	}
	return madeOn, nil
}
