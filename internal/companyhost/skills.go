package companyhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// A bundled skill installs nothing the first time a person uses it: its setup
// keeps what it needs beside its own files, and the install step runs every
// setup as the account that owns the delivered skills, so every person can read
// what was prepared and the next upgrade can replace it.
//
// A skill with a launcher named after it is prepared by `scripts/<skill>
// setup`; any other skill that carries skill_runtime.py by `python3
// scripts/skill_runtime.py setup`; a skill with neither needs nothing. Both
// answer with one envelope and exit non-zero when a piece cannot be prepared.

const (
	skillDocumentName      = "SKILL.md"
	skillRuntimeScriptName = "skill_runtime.py"
	skillSetupArgument     = "setup"
)

type bundledSkill struct {
	Name string
	Path string
}

func (skill bundledSkill) scriptPath(name string) string {
	return filepath.Join(skill.Path, "scripts", name)
}

func (skill bundledSkill) launcherPath() string {
	return skill.scriptPath(skill.Name)
}

type skillSetup struct {
	Program   string
	Arguments []string
}

// setupOf is the command that prepares the skill, if it needs one.
func setupOf(place blueclaw.BundledSkillsPlace, skill bundledSkill) (skillSetup, bool) {
	if isExecutableFile(skill.launcherPath()) {
		return skillSetup{Program: skill.launcherPath(), Arguments: []string{skillSetupArgument}}, true
	}
	if isRegularFile(skill.scriptPath(skillRuntimeScriptName)) {
		return skillSetup{Program: place.PythonPath, Arguments: []string{skill.scriptPath(skillRuntimeScriptName), skillSetupArgument}}, true
	}
	return skillSetup{}, false
}

// skillEnvelope is the result every skill command prints on standard output.
type skillEnvelope struct {
	Status  string       `json:"status"`
	Summary string       `json:"summary"`
	Issues  []skillIssue `json:"issues"`
}

type skillIssue struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}

// SkillSetupRunner starts a skill's setup and hands back what it printed.
type SkillSetupRunner interface {
	Stream(name string, arguments []string, streams Streams) error
}

// LocalProcesses runs a setup as a process of this machine.
type LocalProcesses struct{}

func (LocalProcesses) Stream(name string, arguments []string, streams Streams) error {
	command := exec.Command(name, arguments...)
	command.Stdin = streams.Input
	command.Stdout = streams.Output
	command.Stderr = streams.Errors
	return command.Run()
}

// PrepareTheBundledSkills prepares the skills on this company host.
func PrepareTheBundledSkills(runner SkillSetupRunner, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	return prepareSkillsAt(platform.Layout().BundledSkillsPlace(), runner, progress)
}

// PrepareTheGuestSkills prepares the skills a device delivers to its guest,
// from inside the guest's root filesystem, by the same rule.
func PrepareTheGuestSkills(runner SkillSetupRunner, progress io.Writer) error {
	return prepareSkillsAt(blueclaw.GuestBundledSkillsPlace(), runner, progress)
}

func prepareSkillsAt(place blueclaw.BundledSkillsPlace, runner SkillSetupRunner, progress io.Writer) error {
	syscall.Umask(0o022)
	return prepareSkillsIn(place, runner, progress)
}

func prepareSkillsIn(place blueclaw.BundledSkillsPlace, runner SkillSetupRunner, progress io.Writer) error {
	if errorValue := requireTheOwnerOf(place.SkillsPath); errorValue != nil {
		return errorValue
	}
	if errorValue := removeWhatNoLongerShipsAsASkill(place.SkillsPath, progress); errorValue != nil {
		return errorValue
	}
	skills, errorValue := bundledSkillsIn(place.SkillsPath)
	if errorValue != nil {
		return errorValue
	}
	scratchPath, errorValue := os.MkdirTemp("", "internkim-skill-preparation-")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(scratchPath)
	environment, errorValue := skillPreparationEnvironment(place, scratchPath)
	if errorValue != nil {
		return errorValue
	}
	for _, skill := range skills {
		if errorValue := prepareSkill(place, skill, environment, runner, progress); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

// What is prepared has to be readable by every person and replaceable by the
// next upgrade, so it belongs to whoever owns the skills: root under a Linux
// package, the account that installed the keg on a Mac.
func requireTheOwnerOf(skillsPath string) error {
	information, errorValue := os.Stat(skillsPath)
	if errorValue != nil {
		return fmt.Errorf("no bundled skills at %s: %w", skillsPath, errorValue)
	}
	status, isUnixStatus := information.Sys().(*syscall.Stat_t)
	if !isUnixStatus || int(status.Uid) == os.Geteuid() {
		return nil
	}
	return fmt.Errorf("the skills in %s belong to %s; prepare them as that account so every person can read what is prepared and the next upgrade can replace it",
		skillsPath, accountName(status.Uid))
}

func accountName(userID uint32) string {
	account, errorValue := user.LookupId(fmt.Sprint(userID))
	if errorValue != nil {
		return fmt.Sprintf("uid %d", userID)
	}
	return account.Username
}

// A release that drops a skill leaves its directory behind, holding what its
// setup wrote, which the package manager does not own. A directory without a
// SKILL.md is not a skill, by the rule blueclaw and bundledSkillsIn both read,
// so it goes. A symbolic link is never followed or removed.
func removeWhatNoLongerShipsAsASkill(skillsPath string, progress io.Writer) error {
	entries, errorValue := os.ReadDir(skillsPath)
	if errorValue != nil {
		return errorValue
	}
	for _, entry := range entries {
		leftoverPath := filepath.Join(skillsPath, entry.Name())
		if !entry.Type().IsDir() || isRegularFile(filepath.Join(leftoverPath, skillDocumentName)) {
			continue
		}
		fmt.Fprintf(progress, "Removing %s, which this release no longer ships as a skill…\n", leftoverPath)
		if errorValue := os.RemoveAll(leftoverPath); errorValue != nil {
			return fmt.Errorf("could not remove %s, left by a skill this release no longer ships: %w", leftoverPath, errorValue)
		}
	}
	return nil
}

func bundledSkillsIn(skillsPath string) ([]bundledSkill, error) {
	entries, errorValue := os.ReadDir(skillsPath)
	if errorValue != nil {
		return nil, errorValue
	}
	skills := []bundledSkill{}
	for _, entry := range entries {
		skillPath := filepath.Join(skillsPath, entry.Name())
		if !entry.IsDir() || !isRegularFile(filepath.Join(skillPath, skillDocumentName)) {
			continue
		}
		skills = append(skills, bundledSkill{Name: entry.Name(), Path: skillPath})
	}
	return skills, nil
}

// The skills are prepared on the interpreter and the programs every person's
// command is given, so what a setup builds is something they can run. HOME,
// the cache home and the download caches are this run's alone and go with it,
// so a setup cannot lean on a cache blueclaw hands a person, and only what it
// keeps beside the skill stays.
func skillPreparationEnvironment(place blueclaw.BundledSkillsPlace, scratchPath string) ([]string, error) {
	homePath := filepath.Join(scratchPath, "home")
	if errorValue := os.MkdirAll(homePath, 0o755); errorValue != nil {
		return nil, errorValue
	}
	return []string{
		"PATH=" + place.SearchPath,
		"HOME=" + homePath,
		"XDG_CACHE_HOME=" + filepath.Join(scratchPath, "cache"),
		"UV_CACHE_DIR=" + filepath.Join(scratchPath, "uv"),
		"UV_PYTHON_DOWNLOADS=never",
		"UV_COMPILE_BYTECODE=1",
		"BUN_INSTALL_CACHE_DIR=" + filepath.Join(scratchPath, "bun"),
		"npm_config_cache=" + filepath.Join(scratchPath, "npm"),
	}, nil
}

func prepareSkill(place blueclaw.BundledSkillsPlace, skill bundledSkill, environment []string, runner SkillSetupRunner, progress io.Writer) error {
	setup, needsPreparation := setupOf(place, skill)
	if !needsPreparation {
		return nil
	}
	fmt.Fprintf(progress, "Preparing the %s skill…\n", skill.Name)
	arguments := append(append(append([]string{}, environment...), setup.Program), setup.Arguments...)
	answer := bytes.Buffer{}
	runError := runner.Stream("env", arguments, Streams{Output: &answer, Errors: progress})
	envelope, parseError := parseSkillEnvelope(answer.Bytes())
	if runError == nil {
		fmt.Fprintln(progress, summaryOf(envelope, parseError, answer.String()))
		return nil
	}
	if parseError != nil {
		return fmt.Errorf("could not prepare the %s skill: %s %s failed (%v) and printed %q",
			skill.Name, setup.Program, strings.Join(setup.Arguments, " "), runError, strings.TrimSpace(answer.String()))
	}
	return fmt.Errorf("could not prepare the %s skill: %s", skill.Name, envelope.describe())
}

func parseSkillEnvelope(document []byte) (skillEnvelope, error) {
	envelope := skillEnvelope{}
	if errorValue := json.Unmarshal(document, &envelope); errorValue != nil {
		return skillEnvelope{}, errorValue
	}
	return envelope, nil
}

func summaryOf(envelope skillEnvelope, parseError error, answer string) string {
	if parseError != nil {
		return strings.TrimSpace(answer)
	}
	return envelope.Summary
}

func (envelope skillEnvelope) describe() string {
	lines := []string{envelope.Summary}
	for _, issue := range envelope.Issues {
		lines = append(lines, fmt.Sprintf("  %s: %s %s", issue.Code, issue.Message, issue.Suggestion))
	}
	return strings.Join(lines, "\n")
}

func isRegularFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.Mode().IsRegular()
}

func isExecutableFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.Mode().IsRegular() && information.Mode().Perm()&0o111 != 0
}
