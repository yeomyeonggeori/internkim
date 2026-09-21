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

// A font the skill lists only so Latin text still comes out. It is why a
// missing Korean font is silent rather than an error, and it is the one thing
// the gate must never accept as satisfying the declaration.
var fontPathsThatCarryNoHangul = map[string]string{
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf": "DejaVu has no Hangul; fpdf2 embeds it, drops the glyphs and writes the file anyway",
}

// Where each skill keeps the candidates its resolver walks. These are locators,
// never paths: they carry none of the data they are here to observe, and a name
// that moves makes the script fail to evaluate rather than quietly narrowing
// what this test can see.
var fontCandidateSourceBySkill = map[string]struct{ scriptName, expression string }{
	"pdf":       {"create_pdf.py", "candidate_font_paths()"},
	"paperwork": {"paperwork_design.py", "FONT_CANDIDATE_PATHS_PDF"},
	"document":  {"export_document.py", "cached_font_paths() + PDF_FONT_CANDIDATES"},
}

const fontCandidateProgram = `
import importlib.util
import json
import os
import sys

script_path, expression = sys.argv[1], sys.argv[2]
sys.path.insert(0, os.path.dirname(script_path))
specification = importlib.util.spec_from_file_location("skill_script_under_test", script_path)
module = importlib.util.module_from_spec(specification)
specification.loader.exec_module(module)
print(json.dumps([str(value) for value in eval(expression, vars(module))]))
`

var fontFilePathPattern = regexp.MustCompile(`/[A-Za-z0-9._ /-]+\.(?:ttf|ttc|otf)`)

const requiresAnyFileKey = "kim.intern.requires-any-file"

// Ask the skill which fonts it will walk, rather than reading its source for
// the paths somebody happened to write out in full. A candidate composed from a
// directory and a filename is a candidate.
//
// The cache directory is handed in, so the candidates that live under it can be
// told apart from the ones that are facts about the machine.
func fontCandidatesOf(t *testing.T, skillDirectory SkillDirectory, cacheDirectoryPath string) []string {
	t.Helper()
	source, isKnown := fontCandidateSourceBySkill[skillDirectory.Name]
	if !isKnown {
		t.Fatalf("%s embeds a font into a PDF but this test does not know where its candidates live; add it to fontCandidateSourceBySkill", skillDirectory.Name)
	}
	scriptPath := filepath.Join(skillDirectory.Path, "scripts", source.scriptName)
	command := exec.Command("python3", "-c", fontCandidateProgram, scriptPath, source.expression)
	command.Env = append(os.Environ(), "XDG_CACHE_HOME="+cacheDirectoryPath, "HOME="+cacheDirectoryPath)
	commandOutput, errorValue := command.Output()
	if errorValue != nil {
		t.Fatalf("could not read %s's font candidates through %s: %v\n%s", skillDirectory.Name, source.expression, errorValue, commandOutput)
	}
	candidatePaths := []string{}
	if errorValue := json.Unmarshal(commandOutput, &candidatePaths); errorValue != nil {
		t.Fatalf("%s returned something other than a list of paths: %v\n%s", source.expression, errorValue, commandOutput)
	}
	return candidatePaths
}

func declaredAnyFilePaths(t *testing.T, skillDirectory SkillDirectory) []string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join(skillDirectory.Path, "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, line := range strings.Split(string(document), "\n") {
		_, value, isDeclaration := strings.Cut(strings.TrimSpace(line), requiresAnyFileKey+":")
		if !isDeclaration {
			continue
		}
		return strings.Fields(strings.Trim(strings.TrimSpace(value), `"`))
	}
	return nil
}

// The declaration and the candidate list the script walks are two copies of the
// same fact, and neither can be derived from the other: the script's list ends
// in a Latin-only fallback on purpose, and that fallback is exactly what the
// gate must refuse. So they are written twice and bound here. Add a font to a
// script, by any means, and this fails until you say whether it carries Hangul.
func TestAFontEmbeddingSkillDeclaresEveryCandidateThatCarriesHangul(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	checkedSkillCount := 0
	for _, skillDirectory := range skillsDeclaringRequirements(t, repositoryRootPath) {
		scripts := skillScriptSources(t, skillDirectory)
		if !strings.Contains(scripts, "add_font(") {
			continue
		}
		cacheDirectoryPath := t.TempDir()
		candidatePaths := fontCandidatesOf(t, skillDirectory, cacheDirectoryPath)
		for _, writtenOutPath := range fontFilePathPattern.FindAllString(scripts, -1) {
			if !contains(candidatePaths, writtenOutPath) {
				t.Fatalf("%s names %s in its source but %s does not return it, so this test is watching the wrong list", skillDirectory.Name, writtenOutPath, fontCandidateSourceBySkill[skillDirectory.Name].expression)
			}
		}

		// A cache path is a candidate the skill really uses and the declaration
		// deliberately leaves out. The gate judges the host, and a cache is not a
		// fact about one: it is empty on a fresh install, nothing in either
		// repository fills it, and blueclaw resolves the declaration under its own
		// home rather than the requester's, so declaring it would make a skill's
		// availability depend on whether some earlier task downloaded a font.
		hostCandidatePaths := []string{}
		fallbackReasons := []string{}
		for _, candidatePath := range candidatePaths {
			if strings.HasPrefix(candidatePath, cacheDirectoryPath) {
				continue
			}
			if reason, carriesNoHangul := fontPathsThatCarryNoHangul[candidatePath]; carriesNoHangul {
				fallbackReasons = append(fallbackReasons, candidatePath+" ("+reason+")")
				continue
			}
			if !contains(hostCandidatePaths, candidatePath) {
				hostCandidatePaths = append(hostCandidatePaths, candidatePath)
			}
		}
		declaredPaths := declaredAnyFilePaths(t, skillDirectory)
		checkedSkillCount++

		if len(fallbackReasons) == 0 {
			if len(declaredPaths) > 0 {
				t.Fatalf("%s walks no Latin-only fallback, so a missing Korean font already stops it; declaring %v hides a skill that would have said so itself", skillDirectory.Name, declaredPaths)
			}
			continue
		}
		if len(declaredPaths) == 0 {
			t.Fatalf("%s falls back to %s, so a missing Korean font is silent there; it must declare %s with the candidates that carry Hangul: %v", skillDirectory.Name, strings.Join(fallbackReasons, ", "), requiresAnyFileKey, hostCandidatePaths)
		}
		for _, candidatePath := range hostCandidatePaths {
			if !contains(declaredPaths, candidatePath) {
				t.Fatalf("%s walks %s but does not declare it, so the gate would hide the skill on a host where that font is the one installed", skillDirectory.Name, candidatePath)
			}
		}
		for _, declaredPath := range declaredPaths {
			if !contains(hostCandidatePaths, declaredPath) {
				t.Fatalf("%s declares %s, which it never walks; the gate would require a font the skill would never use", skillDirectory.Name, declaredPath)
			}
		}
	}
	if checkedSkillCount < 2 {
		t.Fatalf("expected several skills to embed a font into a PDF, found %d", checkedSkillCount)
	}
}

// The image promises one font and the skills accept a list; a promise outside
// that list is a declaration nobody satisfies, and the skill would be withheld
// on a host that has everything it was built to have.
func TestTheFontTheHostImageGuaranteesIsOneTheDeclarationsAccept(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	guaranteedFontPath := shellAssignment(hostEntrypoint(t, repositoryRootPath), "koreanCapableFontPath")
	if guaranteedFontPath == "" {
		t.Fatal("host entrypoint must name the Korean-capable font path it checks for")
	}
	declaringSkillCount := 0
	for _, skillDirectory := range skillsDeclaringRequirements(t, repositoryRootPath) {
		declaredPaths := declaredAnyFilePaths(t, skillDirectory)
		if len(declaredPaths) == 0 {
			continue
		}
		declaringSkillCount++
		if !contains(declaredPaths, guaranteedFontPath) {
			t.Fatalf("the image guarantees %s and %s accepts only %v, so the skill would be withheld on a complete host", guaranteedFontPath, skillDirectory.Name, declaredPaths)
		}
	}
	if declaringSkillCount == 0 {
		t.Fatal("expected the skills whose missing font is silent to declare the fonts that would do")
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
