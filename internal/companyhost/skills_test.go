package companyhost

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	readySetupAnswer          = `{"status": "ok", "summary": "ready", "issues": []}`
	unknownSetupCommandAnswer = `{"status": "error", "summary": "unknown command 'setup'", "issues": [{"code": "UNKNOWN_COMMAND", "message": "unknown command 'setup'", "suggestion": "run --help"}]}`
	noRendererAnswer          = `{"status": "error", "summary": "the renderer cannot be prepared", "issues": [{"code": "NO_JAVASCRIPT_RUNTIME", "message": "neither bun nor node 18 is on PATH", "suggestion": "install bun"}]}`
)

type skillFixture struct {
	hasSetupEntry bool
	requirements  string
	hasRuntime    bool
	isNotASkill   bool
}

func skillsLayout(t *testing.T, fixtures map[string]skillFixture) blueclaw.CompanyHostLayout {
	t.Helper()
	root := t.TempDir()
	layout := blueclaw.CompanyHostLayout{
		BinaryRoot:         filepath.Join(root, "bin"),
		LibraryRoot:        filepath.Join(root, "lib"),
		ProgramDirectories: []string{filepath.Join(root, "bin"), "/usr/bin"},
	}
	for name, fixture := range fixtures {
		scriptsPath := filepath.Join(layout.SkillsPath(), name, "scripts")
		writeFixtureFile(t, filepath.Join(scriptsPath, "placeholder"), "", 0o644)
		if !fixture.isNotASkill {
			writeFixtureFile(t, filepath.Join(layout.SkillsPath(), name, skillDocumentName), "---\nname: "+name+"\n---\n", 0o644)
		}
		if fixture.hasSetupEntry {
			writeFixtureFile(t, filepath.Join(scriptsPath, name), "#!/bin/sh\n", 0o755)
		}
		if fixture.hasRuntime {
			writeFixtureFile(t, filepath.Join(scriptsPath, skillRuntimeScriptName), "", 0o644)
		}
		if fixture.requirements != "" || fixture.hasRuntime {
			writeFixtureFile(t, filepath.Join(scriptsPath, skillRequirementsFileName), fixture.requirements, 0o644)
		}
	}
	return layout
}

func writeFixtureFile(t *testing.T, path string, content string, mode os.FileMode) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), mode); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func setupRuns(machine *recordedMachine, layout blueclaw.CompanyHostLayout, skillName string) [][]string {
	entryPath := filepath.Join(layout.SkillsPath(), skillName, "scripts", skillName)
	runs := [][]string{}
	for _, run := range machine.runs {
		if run[0] == "env" && slices.Contains(run, entryPath) {
			runs = append(runs, run)
		}
	}
	return runs
}

func bootstrapRuns(machine *recordedMachine, layout blueclaw.CompanyHostLayout, skillName string) [][]string {
	runtimePath := filepath.Join(layout.SkillsPath(), skillName, "scripts", skillRuntimeScriptName)
	runs := [][]string{}
	for _, run := range machine.runs {
		if run[0] == layout.PythonPath() && len(run) > 1 && run[1] == runtimePath {
			runs = append(runs, run)
		}
	}
	return runs
}

func TestEveryBundledSkillIsPreparedTheWayItDeclares(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office":     {hasSetupEntry: true, hasRuntime: true, requirements: "python-docx\n"},
		"dataroom":   {hasRuntime: true, requirements: "pyyaml\n"},
		"calculator": {},
		"weather":    {hasRuntime: true, requirements: "\n"},
		"stray":      {hasSetupEntry: true, isNotASkill: true},
	})
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, nil, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(setupRuns(machine, layout, "office")) != 1 || len(bootstrapRuns(machine, layout, "office")) != 0 {
		t.Fatalf("a skill with a setup of its own is prepared by that setup alone: %v", machine.runs)
	}
	if len(bootstrapRuns(machine, layout, "dataroom")) != 1 {
		t.Fatalf("a skill that declares requirements and has no setup is not prepared by its skill_runtime.py: %v", machine.runs)
	}
	if len(machine.runs) != 2 {
		t.Fatalf("a skill that declares nothing, or a directory that is not a skill, was prepared: %v", machine.runs)
	}
}

func TestSkillsArePreparedOnWhatEveryPersonsCommandRunsWithNoCacheOfTheirs(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office":   {hasSetupEntry: true},
		"dataroom": {hasRuntime: true, requirements: "pyyaml\n"},
	})
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, nil, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	runs := append(setupRuns(machine, layout, "office"), bootstrapRuns(machine, layout, "dataroom")...)
	if len(runs) != 2 {
		t.Fatalf("expected the setup and the bootstrap to run, ran %v", machine.runs)
	}
	for _, run := range runs {
		for _, setting := range []string{"PATH=" + layout.SearchPath(), "UV_PYTHON_DOWNLOADS=never"} {
			if !slices.Contains(run, setting) {
				t.Fatalf("%v does not run with %s", run, setting)
			}
		}
		for _, name := range []string{"HOME", "XDG_CACHE_HOME", "UV_CACHE_DIR", "BUN_INSTALL_CACHE_DIR", "npm_config_cache"} {
			value := settingIn(run, name)
			if value == "" || !strings.Contains(value, "internkim-skill-preparation-") {
				t.Fatalf("%v runs with %s=%q, a directory that outlives this run", run, name, value)
			}
			if _, errorValue := os.Stat(value); !os.IsNotExist(errorValue) {
				t.Fatalf("%s=%s was left behind after the preparation", name, value)
			}
		}
	}
}

func settingIn(run []string, name string) string {
	for _, argument := range run {
		if value, isSetting := strings.CutPrefix(argument, name+"="); isSetting {
			return value
		}
	}
	return ""
}

func TestASkillTheReleaseNoLongerShipsIsRemovedBeforeTheOthersArePrepared(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"dataroom": {hasRuntime: true, requirements: "pyyaml\n"},
		"website":  {hasSetupEntry: true, isNotASkill: true},
	})
	leftoverPath := filepath.Join(layout.SkillsPath(), "website")
	outsidePath := filepath.Join(t.TempDir(), "outside")
	writeFixtureFile(t, filepath.Join(outsidePath, "kept"), "", 0o644)
	linkPath := filepath.Join(layout.SkillsPath(), "linked")
	if errorValue := os.Symlink(outsidePath, linkPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, nil, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(leftoverPath); !os.IsNotExist(errorValue) {
		t.Fatalf("%s, a directory with no SKILL.md, survived the upgrade: %v", leftoverPath, errorValue)
	}
	if len(bootstrapRuns(machine, layout, "dataroom")) != 1 || len(setupRuns(machine, layout, "website")) != 0 {
		t.Fatalf("expected only the shipped skill to be prepared: %v", machine.runs)
	}
	if _, errorValue := os.Lstat(linkPath); errorValue != nil {
		t.Fatalf("a symbolic link under the skills was removed: %v", errorValue)
	}
	if _, errorValue := os.Stat(filepath.Join(outsidePath, "kept")); errorValue != nil {
		t.Fatalf("removal reached outside the skills through a link: %v", errorValue)
	}
}

func TestASkillWhoseCommandPredatesSetupIsPreparedFromItsRequirements(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office": {hasSetupEntry: true, hasRuntime: true, requirements: "python-docx\n"},
	})
	machine := &recordedMachine{
		printed:  map[string]string{"env": unknownSetupCommandAnswer},
		failures: map[string]error{"env": errors.New("exit status 1")},
	}
	if errorValue := prepareSkillsIn(layout, nil, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(bootstrapRuns(machine, layout, "office")) != 1 {
		t.Fatalf("a command that does not know setup left its skill unprepared: %v", machine.runs)
	}
}

func TestASetupThatFailsStopsTheInstallNamingWhatItNeeds(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office": {hasSetupEntry: true, hasRuntime: true, requirements: "python-docx\n"},
	})
	machine := &recordedMachine{
		printed:  map[string]string{"env": noRendererAnswer},
		failures: map[string]error{"env": errors.New("exit status 1")},
	}
	errorValue := prepareSkillsIn(layout, nil, machine, &strings.Builder{})
	if errorValue == nil {
		t.Fatal("a skill whose setup failed was reported prepared")
	}
	for _, named := range []string{"office", "NO_JAVASCRIPT_RUNTIME", "neither bun nor node 18 is on PATH", "install bun"} {
		if !strings.Contains(errorValue.Error(), named) {
			t.Fatalf("the failure does not say %q: %v", named, errorValue)
		}
	}
	if len(bootstrapRuns(machine, layout, "office")) != 0 {
		t.Fatalf("a setup that failed was papered over with a partial environment: %v", machine.runs)
	}
}

func TestAFailedEnvironmentStopsTheInstallNamingTheSkill(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"dataroom": {hasRuntime: true, requirements: "pyyaml\n"},
	})
	machine := &recordedMachine{failures: map[string]error{layout.PythonPath(): errors.New("exit status 1")}}
	errorValue := prepareSkillsIn(layout, nil, machine, &strings.Builder{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "dataroom") {
		t.Fatalf("a skill whose environment could not be made was reported prepared: %v", errorValue)
	}
}

func TestSetupOptionsReachEverySetupAndNothingElse(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office":   {hasSetupEntry: true},
		"dataroom": {hasRuntime: true, requirements: "pyyaml\n"},
	})
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, []string{"--with-ocr"}, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	setup := setupRuns(machine, layout, "office")
	if len(setup) != 1 || !slices.Equal(setup[0][len(setup[0])-2:], []string{skillSetupArgument, "--with-ocr"}) {
		t.Fatalf("the option did not reach the setup: %v", setup)
	}
	for _, run := range bootstrapRuns(machine, layout, "dataroom") {
		if slices.Contains(run, "--with-ocr") {
			t.Fatalf("a setup option was handed to a skill with no setup: %v", run)
		}
	}
}

// The rule above has to reach every skill the package actually ships, or the
// first person to use one still waits for an install.
func TestEverySkillThePluginShipsWithRequirementsHasAWayToBePrepared(t *testing.T) {
	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(filepath.Join("..", ".."))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	preparable := 0
	for _, skillRootPath := range skillRootPaths {
		skills, errorValue := bundledSkillsIn(skillRootPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		for _, skill := range skills {
			if !isRegularFile(skill.requirementsPath()) {
				continue
			}
			if !isExecutableFile(skill.setupEntryPath()) && !declaresPythonRequirements(skill) {
				t.Errorf("%s declares %s, and has neither scripts/%s nor scripts/%s to prepare it with", skill.Name, skillRequirementsFileName, skill.Name, skillRuntimeScriptName)
			}
			preparable++
		}
	}
	if preparable == 0 {
		t.Fatal("no shipped skill declares requirements; the plugin moved them somewhere this rule does not look")
	}
}
