package companyhost

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/blueclawworkspace"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	readySetupAnswer  = `{"status": "ok", "summary": "ready", "issues": []}`
	failedSetupAnswer = `{"status": "error", "summary": "the renderer cannot be prepared", "issues": [{"code": "SETUP_FAILED", "message": "neither bun nor node 18 is on PATH", "suggestion": "install bun"}]}`
)

type skillFixture struct {
	hasLauncher bool
	hasRuntime  bool
	isNotASkill bool
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
		if fixture.hasLauncher {
			writeFixtureFile(t, filepath.Join(scriptsPath, name), "#!/bin/sh\n", 0o755)
		}
		if fixture.hasRuntime {
			writeFixtureFile(t, filepath.Join(scriptsPath, skillRuntimeScriptName), "", 0o644)
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

func runsEndingWith(machine *recordedMachine, ending ...string) [][]string {
	runs := [][]string{}
	for _, run := range machine.runs {
		if run[0] == "env" && len(run) >= len(ending) && slices.Equal(run[len(run)-len(ending):], ending) {
			runs = append(runs, run)
		}
	}
	return runs
}

func launcherSetupRuns(machine *recordedMachine, layout blueclaw.CompanyHostLayout, skillName string) [][]string {
	launcherPath := filepath.Join(layout.SkillsPath(), skillName, "scripts", skillName)
	return runsEndingWith(machine, launcherPath, skillSetupArgument)
}

func runtimeSetupRuns(machine *recordedMachine, layout blueclaw.CompanyHostLayout, skillName string) [][]string {
	runtimePath := filepath.Join(layout.SkillsPath(), skillName, "scripts", skillRuntimeScriptName)
	return runsEndingWith(machine, layout.PythonPath(), runtimePath, skillSetupArgument)
}

func TestEveryBundledSkillIsPreparedByItsOwnSetup(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office":     {hasLauncher: true, hasRuntime: true},
		"dataroom":   {hasRuntime: true},
		"calculator": {},
		"stray":      {hasLauncher: true, isNotASkill: true},
	})
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(launcherSetupRuns(machine, layout, "office")) != 1 || len(runtimeSetupRuns(machine, layout, "office")) != 0 {
		t.Fatalf("a skill with a launcher is prepared by the launcher's setup alone: %v", machine.runs)
	}
	if len(runtimeSetupRuns(machine, layout, "dataroom")) != 1 {
		t.Fatalf("a skill without a launcher is not prepared by `python3 scripts/skill_runtime.py setup`: %v", machine.runs)
	}
	if len(machine.runs) != 2 {
		t.Fatalf("a skill with neither, or a directory that is not a skill, was prepared: %v", machine.runs)
	}
}

func TestSkillsArePreparedOnWhatEveryPersonsCommandRunsWithNoCacheOfTheirs(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{
		"office":   {hasLauncher: true},
		"dataroom": {hasRuntime: true},
	})
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	runs := append(launcherSetupRuns(machine, layout, "office"), runtimeSetupRuns(machine, layout, "dataroom")...)
	if len(runs) != 2 {
		t.Fatalf("expected both setups to run, ran %v", machine.runs)
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
		"dataroom": {hasRuntime: true},
		"website":  {hasLauncher: true, isNotASkill: true},
	})
	leftoverPath := filepath.Join(layout.SkillsPath(), "website")
	outsidePath := filepath.Join(t.TempDir(), "outside")
	writeFixtureFile(t, filepath.Join(outsidePath, "kept"), "", 0o644)
	linkPath := filepath.Join(layout.SkillsPath(), "linked")
	if errorValue := os.Symlink(outsidePath, linkPath); errorValue != nil {
		t.Fatal(errorValue)
	}
	machine := &recordedMachine{printed: map[string]string{"env": readySetupAnswer}}
	if errorValue := prepareSkillsIn(layout, machine, &strings.Builder{}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := os.Stat(leftoverPath); !os.IsNotExist(errorValue) {
		t.Fatalf("%s, a directory with no SKILL.md, survived the upgrade: %v", leftoverPath, errorValue)
	}
	if len(runtimeSetupRuns(machine, layout, "dataroom")) != 1 || len(machine.runs) != 1 {
		t.Fatalf("expected only the shipped skill to be prepared: %v", machine.runs)
	}
	if _, errorValue := os.Lstat(linkPath); errorValue != nil {
		t.Fatalf("a symbolic link under the skills was removed: %v", errorValue)
	}
	if _, errorValue := os.Stat(filepath.Join(outsidePath, "kept")); errorValue != nil {
		t.Fatalf("removal reached outside the skills through a link: %v", errorValue)
	}
}

func TestASetupThatFailsStopsTheInstallNamingWhatItNeeds(t *testing.T) {
	for _, fixture := range map[string]skillFixture{"office": {hasLauncher: true}, "dataroom": {hasRuntime: true}} {
		layout := skillsLayout(t, map[string]skillFixture{"skill": fixture})
		machine := &recordedMachine{
			printed:  map[string]string{"env": failedSetupAnswer},
			failures: map[string]error{"env": errors.New("exit status 1")},
		}
		errorValue := prepareSkillsIn(layout, machine, &strings.Builder{})
		if errorValue == nil {
			t.Fatal("a skill whose setup failed was reported prepared")
		}
		for _, named := range []string{"skill", "SETUP_FAILED", "neither bun nor node 18 is on PATH", "install bun"} {
			if !strings.Contains(errorValue.Error(), named) {
				t.Fatalf("the failure does not say %q: %v", named, errorValue)
			}
		}
	}
}

func TestASetupThatPrintsNoEnvelopeStopsTheInstall(t *testing.T) {
	layout := skillsLayout(t, map[string]skillFixture{"office": {hasLauncher: true}})
	machine := &recordedMachine{
		printed:  map[string]string{"env": "Traceback (most recent call last):"},
		failures: map[string]error{"env": errors.New("exit status 1")},
	}
	errorValue := prepareSkillsIn(layout, machine, &strings.Builder{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "Traceback") {
		t.Fatalf("a setup that crashed was reported prepared: %v", errorValue)
	}
}

// The rule above has to reach every skill the package actually ships, or the
// first person to use one still waits for an install.
func TestEverySkillThePluginShipsThatDeclaresPackagesHasASetupToRun(t *testing.T) {
	skillRootPaths, errorValue := blueclawworkspace.SkillRootPaths(filepath.Join("..", ".."))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	layout := blueclaw.LinuxCompanyHostLayout()
	declaring := 0
	for _, skillRootPath := range skillRootPaths {
		skills, errorValue := bundledSkillsIn(skillRootPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		for _, skill := range skills {
			manifest := dependencyManifestIn(t, skill)
			if manifest == "" {
				continue
			}
			declaring++
			if _, hasSetup := setupOf(layout, skill); !hasSetup {
				t.Errorf("%s declares %s and has neither scripts/%s nor scripts/%s to set it up with", skill.Name, manifest, skill.Name, skillRuntimeScriptName)
			}
		}
	}
	if declaring == 0 {
		t.Fatal("no shipped skill declares requirements; the plugin moved them somewhere this rule does not look")
	}
}

// dependencyManifestIn names the first file in the skill that declares
// packages to install, in any of the formats the skills use.
func dependencyManifestIn(t *testing.T, skill bundledSkill) string {
	t.Helper()
	manifestNames := map[string]bool{"requirements.txt": true, "pylock.toml": true, "package.json": true}
	found := ""
	walkError := filepath.WalkDir(skill.Path, func(path string, entry fs.DirEntry, walkError error) error {
		if walkError != nil || found != "" {
			return walkError
		}
		if entry.IsDir() && entry.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if !entry.IsDir() && manifestNames[entry.Name()] {
			found, _ = filepath.Rel(skill.Path, path)
		}
		return nil
	})
	if walkError != nil {
		t.Fatal(walkError)
	}
	return found
}
