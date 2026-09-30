package blueclawworkspace

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var fontPathsThatCarryNoHangul = map[string]string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf": "DejaVu has no Hangul; fpdf2 embeds it, drops the glyphs and writes the file anyway",
}

type fontCandidateSource struct {
	scriptName string
	expression string
}

var fontCandidateSourcesByFormat = []fontCandidateSource{
	{"pdf/create_pdf.py", "candidate_font_paths()"},
	{"paperwork/paperwork_design.py", "FONT_CANDIDATE_PATHS_PDF"},
	{"doc/export_document.py", "cached_font_paths() + PDF_FONT_CANDIDATES"},
}

const fontCandidateProgram = `
import importlib.util
import json
import os
import sys

script_path, expression, runtime_directory = sys.argv[1], sys.argv[2], sys.argv[3]
sys.path.insert(0, runtime_directory)
sys.path.insert(0, os.path.dirname(script_path))
specification = importlib.util.spec_from_file_location("skill_script_under_test", script_path)
module = importlib.util.module_from_spec(specification)
specification.loader.exec_module(module)
print(json.dumps([str(value) for value in eval(expression, vars(module))]))
`

var fontFilePathPattern = regexp.MustCompile(`/[A-Za-z0-9._ /-]+\.(?:ttf|ttc|otf)`)

const requiresAnyFileKey = "kim.intern.requires-any-file"

func officeSkillDirectory(t *testing.T, repositoryRootPath string) SkillDirectory {
	t.Helper()
	requirePluginSkills(t, repositoryRootPath)
	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillDirectory := range skillDirectories {
		if skillDirectory.Name == "office" {
			return skillDirectory
		}
	}
	t.Fatal("no plugin carries the office skill")
	return SkillDirectory{}
}

func fontCandidatesOf(t *testing.T, skillDirectory SkillDirectory, source fontCandidateSource, cacheDirectoryPath string) []string {
	t.Helper()
	runtimeDirectory := filepath.Join(skillDirectory.Path, "scripts")
	scriptPath := filepath.Join(runtimeDirectory, source.scriptName)
	command := exec.Command("python3", "-c", fontCandidateProgram, scriptPath, source.expression, runtimeDirectory)
	command.Env = append(os.Environ(), "XDG_CACHE_HOME="+cacheDirectoryPath, "HOME="+cacheDirectoryPath)
	commandOutput, errorValue := command.Output()
	if errorValue != nil {
		t.Fatalf("could not read %s's font candidates through %s: %v\n%s", source.scriptName, source.expression, errorValue, commandOutput)
	}
	candidatePaths := []string{}
	if errorValue := json.Unmarshal(commandOutput, &candidatePaths); errorValue != nil {
		t.Fatalf("%s returned something other than a list of paths: %v\n%s", source.expression, errorValue, commandOutput)
	}
	return candidatePaths
}

func hostFontCandidatesOf(t *testing.T, skillDirectory SkillDirectory, source fontCandidateSource) []string {
	t.Helper()
	cacheDirectoryPath := t.TempDir()
	hostCandidatePaths := []string{}
	for _, candidatePath := range fontCandidatesOf(t, skillDirectory, source, cacheDirectoryPath) {
		if strings.HasPrefix(candidatePath, cacheDirectoryPath) {
			continue
		}
		if _, carriesNoHangul := fontPathsThatCarryNoHangul[candidatePath]; carriesNoHangul {
			continue
		}
		hostCandidatePaths = append(hostCandidatePaths, candidatePath)
	}
	return hostCandidatePaths
}

func frontMatterOf(t *testing.T, skillDirectory SkillDirectory) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join(skillDirectory.Path, "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	frontMatter, _, _ := strings.Cut(strings.TrimPrefix(string(document), "---\n"), "\n---\n")
	return frontMatter
}

func TestTheOfficeSkillStatesItsKoreanPDFFontsInCompatibility(t *testing.T) {
	skillDirectory := officeSkillDirectory(t, filepath.Join("..", ".."))
	frontMatter := frontMatterOf(t, skillDirectory)
	if strings.Contains(frontMatter, requiresAnyFileKey) {
		t.Fatalf("the office skill must not declare %s: a missing Korean font would hide spreadsheets and decks on a host that can still write them", requiresAnyFileKey)
	}
	if !strings.Contains(frontMatter, "Korean PDFs need a Korean-capable TTF or TTC font") {
		t.Fatal("the office skill must state in compatibility that Korean PDFs need a Korean-capable TTF or TTC font")
	}
}

func TestEveryOfficePDFFontListHoldsEveryFontItsSourceWritesOutInFull(t *testing.T) {
	skillDirectory := officeSkillDirectory(t, filepath.Join("..", ".."))
	runtimeScript, errorValue := os.ReadFile(filepath.Join(skillDirectory.Path, "scripts", "skill_runtime.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, source := range fontCandidateSourcesByFormat {
		script, errorValue := os.ReadFile(filepath.Join(skillDirectory.Path, "scripts", source.scriptName))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		candidatePaths := fontCandidatesOf(t, skillDirectory, source, t.TempDir())
		for _, writtenOutPath := range fontFilePathPattern.FindAllString(string(script)+string(runtimeScript), -1) {
			if !contains(candidatePaths, writtenOutPath) {
				t.Fatalf("%s names %s in its source but %s does not return it, so this test is watching the wrong list", source.scriptName, writtenOutPath, source.expression)
			}
		}
		if len(hostFontCandidatesOf(t, skillDirectory, source)) == 0 {
			t.Fatalf("%s walks no font that carries Hangul", source.scriptName)
		}
	}
}

func TestTheFontTheHostImageGuaranteesIsOneTheOfficeSkillWalks(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	guaranteedFontPath := shellAssignment(hostEntrypoint(t, repositoryRootPath), "koreanCapableFontPath")
	if guaranteedFontPath == "" {
		t.Fatal("host entrypoint must name the Korean-capable font path it checks for")
	}
	skillDirectory := officeSkillDirectory(t, repositoryRootPath)
	for _, source := range fontCandidateSourcesByFormat {
		if !contains(hostFontCandidatesOf(t, skillDirectory, source), guaranteedFontPath) {
			t.Fatalf("the image guarantees %s and %s never looks there", guaranteedFontPath, source.scriptName)
		}
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
