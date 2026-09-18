package browserextension

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallUnpacksTheExtensionAndLeavesUnchangedFilesAlone(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "browser-extension")
	installed, errorValue := Install(directory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if installed != directory {
		t.Fatalf("installed at %s", installed)
	}
	for _, name := range []string{"manifest.json", "background.js", "content.js"} {
		if _, errorValue := os.Stat(filepath.Join(directory, name)); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	manifestPath := filepath.Join(directory, "manifest.json")
	before, _ := os.Stat(manifestPath)
	if _, errorValue := Install(directory); errorValue != nil {
		t.Fatal(errorValue)
	}
	after, _ := os.Stat(manifestPath)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("an unchanged file was rewritten")
	}
}
