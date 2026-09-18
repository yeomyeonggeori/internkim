package browserextension

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed manifest.json background.js content.js
var files embed.FS

func Install(directory string) (string, error) {
	if errorValue := os.MkdirAll(directory, 0o755); errorValue != nil {
		return "", errorValue
	}
	entries, errorValue := fs.ReadDir(files, ".")
	if errorValue != nil {
		return "", errorValue
	}
	for _, entry := range entries {
		if errorValue := installFile(directory, entry.Name()); errorValue != nil {
			return "", errorValue
		}
	}
	return directory, nil
}

func installFile(directory string, name string) error {
	content, errorValue := files.ReadFile(name)
	if errorValue != nil {
		return errorValue
	}
	path := filepath.Join(directory, name)
	if existing, readError := os.ReadFile(path); readError == nil && bytes.Equal(existing, content) {
		return nil
	}
	return os.WriteFile(path, content, 0o644)
}
