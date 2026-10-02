package blueclawworkspace

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// A skill's packages are declared once, in its own scripts/requirements.txt,
// and the install step prepares them from there. A list copied into this
// repository drifts from the plugin without anything failing: the device's
// rootfs once carried one that named packages no skill used and missed two
// that one did. The conversion environment is capabilityd's own and no skill
// runs in it.
func TestOnlyThePluginDeclaresTheSkillsPackages(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	conversionLock := filepath.Join("assets", "document-conversion")
	skippedDirectoryNames := map[string]bool{".git": true, ".dependency": true, ".local": true, "node_modules": true, ".svelte-kit": true}
	walkError := filepath.WalkDir(repositoryRootPath, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() && skippedDirectoryNames[entry.Name()] {
			return filepath.SkipDir
		}
		if entry.IsDir() || !isRequirementsList(entry.Name()) {
			return nil
		}
		relativePath, _ := filepath.Rel(repositoryRootPath, path)
		if filepath.Dir(relativePath) != conversionLock {
			t.Errorf("%s lists Python packages outside the plugin; a skill declares its own in scripts/requirements.txt and `internkim %s` prepares it from there",
				relativePath, blueclaw.SkillPreparationVerb)
		}
		return nil
	})
	if walkError != nil {
		t.Fatal(walkError)
	}
}

func isRequirementsList(fileName string) bool {
	extension := filepath.Ext(fileName)
	if extension != ".txt" && extension != ".in" {
		return false
	}
	return strings.HasSuffix(strings.TrimSuffix(fileName, extension), "requirements")
}
