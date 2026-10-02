package blueclawworkspace

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

var systemFontPathPattern = regexp.MustCompile(`/usr/share/fonts/[A-Za-z0-9._/-]+`)

func skillScriptSources(t *testing.T, skillDirectory SkillDirectory) string {
	t.Helper()
	sources := strings.Builder{}
	walkError := filepath.WalkDir(filepath.Join(skillDirectory.Path, "scripts"), func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil || entry.IsDir() || filepath.Ext(path) != ".py" {
			return walkError
		}
		script, readError := os.ReadFile(path)
		if readError != nil {
			return readError
		}
		sources.Write(script)
		return nil
	})
	if walkError != nil {
		t.Fatal(walkError)
	}
	return sources.String()
}

func skillsDeclaringRequirements(t *testing.T, repositoryRootPath string) []SkillDirectory {
	t.Helper()
	requirePluginSkills(t, repositoryRootPath)
	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	declaring := []SkillDirectory{}
	for _, skillDirectory := range skillDirectories {
		if _, statError := os.Stat(filepath.Join(skillDirectory.Path, "scripts", "requirements.txt")); statError == nil {
			declaring = append(declaring, skillDirectory)
		}
	}
	return declaring
}

func TestThePackageCarriesTheFontEveryFontEmbeddingSkillLooksFor(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	sharedFontPaths := map[string]bool{}
	embeddingSkillCount := 0
	for _, skillDirectory := range skillsDeclaringRequirements(t, repositoryRootPath) {
		scripts := skillScriptSources(t, skillDirectory)
		if !strings.Contains(scripts, "add_font(") {
			continue
		}
		declared := map[string]bool{}
		for _, fontPath := range systemFontPathPattern.FindAllString(scripts, -1) {
			if _, carriesNoHangul := fontPathsThatCarryNoHangul[fontPath]; carriesNoHangul {
				continue
			}
			declared[fontPath] = true
		}
		if len(declared) == 0 {
			t.Fatalf("%s embeds a font into a PDF but names no system font path that carries Hangul", skillDirectory.Name)
		}
		embeddingSkillCount++
		if embeddingSkillCount == 1 {
			sharedFontPaths = declared
			continue
		}
		for fontPath := range sharedFontPaths {
			if !declared[fontPath] {
				delete(sharedFontPaths, fontPath)
			}
		}
	}
	if embeddingSkillCount < 1 {
		t.Fatal("expected a skill to embed a system font into a PDF")
	}
	if len(sharedFontPaths) == 0 {
		t.Fatal("the skills that embed a font no longer share a system path; the package cannot satisfy them with one font")
	}

	if !sharedFontPaths[blueclaw.CompanyPackageDocumentFontPath] {
		t.Fatalf("the .deb carries its Hangul font at %s and not every font-embedding skill looks there (they share %s)",
			blueclaw.CompanyPackageDocumentFontPath, strings.Join(sortedKeys(sharedFontPaths), ", "))
	}
}

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

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
