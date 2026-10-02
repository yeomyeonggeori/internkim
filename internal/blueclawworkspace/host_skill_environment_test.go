package blueclawworkspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestThePackagePinsTheUVReleaseTheDeviceRootfsInstalls(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	hostVersion := packagePinnedVersion(t, blueclaw.PackageResolverName)
	prepareScript, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	devicePin := regexp.MustCompile(`blueclaw_uv_version="\$\{BLUECLAW_UV_VERSION:-([0-9.]+)\}"`).FindStringSubmatch(string(prepareScript))
	if devicePin == nil {
		t.Fatal("prepare-blueclaw-runtime no longer pins a uv version the host can match")
	}
	if hostVersion != devicePin[1] {
		t.Fatalf("the package installs uv %s while the device rootfs installs %s; one skill environment, one release", hostVersion, devicePin[1])
	}
}

func packagePinnedVersion(t *testing.T, programName string) string {
	t.Helper()
	pins, errorValue := blueclaw.HostPayloadDownloads("arm64")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, pin := range pins {
		if pin.ProgramName == programName {
			return pin.Version
		}
	}
	t.Fatalf("the package pins no %s", programName)
	return ""
}

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
