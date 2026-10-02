package companyhost

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
// A skill whose command line is named after it is prepared by that command's
// `setup`. Until the pinned plugin carries one, a command that answers `setup`
// with UNKNOWN_COMMAND is prepared by its skill_runtime.py bootstrap instead,
// which keeps its environment in the caller's cache home; here that is this
// run's scratch directory, so the bootstrap shows the requirements resolve and
// keeps nothing.

const (
	skillDocumentName         = "SKILL.md"
	skillRuntimeScriptName    = "skill_runtime.py"
	skillRequirementsFileName = "requirements.txt"
	skillSetupArgument        = "setup"
	unknownCommandIssueCode   = "UNKNOWN_COMMAND"
)

type bundledSkill struct {
	Name string
	Path string
}

func (skill bundledSkill) scriptPath(name string) string {
	return filepath.Join(skill.Path, "scripts", name)
}

func (skill bundledSkill) setupEntryPath() string {
	return skill.scriptPath(skill.Name)
}

func (skill bundledSkill) requirementsPath() string {
	return skill.scriptPath(skillRequirementsFileName)
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

// PrepareTheBundledSkills prepares the skills on this machine. The options are
// handed to every skill's setup unchanged.
func PrepareTheBundledSkills(setupOptions []string, machine Machine, progress io.Writer) error {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return errorValue
	}
	syscall.Umask(0o022)
	return prepareSkillsIn(platform.Layout(), setupOptions, machine, progress)
}

func prepareSkillsIn(layout blueclaw.CompanyHostLayout, setupOptions []string, machine Machine, progress io.Writer) error {
	if errorValue := requireTheOwnerOf(layout.SkillsPath()); errorValue != nil {
		return errorValue
	}
	if errorValue := removeWhatNoLongerShipsAsASkill(layout.SkillsPath(), progress); errorValue != nil {
		return errorValue
	}
	skills, errorValue := bundledSkillsIn(layout.SkillsPath())
	if errorValue != nil {
		return errorValue
	}
	scratchPath, errorValue := os.MkdirTemp("", "internkim-skill-preparation-")
	if errorValue != nil {
		return errorValue
	}
	defer os.RemoveAll(scratchPath)
	environment, errorValue := skillPreparationEnvironment(layout, scratchPath)
	if errorValue != nil {
		return errorValue
	}
	for _, skill := range skills {
		if errorValue := prepareSkill(layout, skill, setupOptions, environment, machine, progress); errorValue != nil {
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
func skillPreparationEnvironment(layout blueclaw.CompanyHostLayout, scratchPath string) ([]string, error) {
	homePath := filepath.Join(scratchPath, "home")
	if errorValue := os.MkdirAll(homePath, 0o755); errorValue != nil {
		return nil, errorValue
	}
	return []string{
		"PATH=" + layout.SearchPath(),
		"HOME=" + homePath,
		"XDG_CACHE_HOME=" + filepath.Join(scratchPath, "cache"),
		"UV_CACHE_DIR=" + filepath.Join(scratchPath, "uv"),
		"UV_PYTHON_DOWNLOADS=never",
		"UV_COMPILE_BYTECODE=1",
		"BUN_INSTALL_CACHE_DIR=" + filepath.Join(scratchPath, "bun"),
		"npm_config_cache=" + filepath.Join(scratchPath, "npm"),
	}, nil
}

func prepareSkill(layout blueclaw.CompanyHostLayout, skill bundledSkill, setupOptions []string, environment []string, machine Machine, progress io.Writer) error {
	if isExecutableFile(skill.setupEntryPath()) {
		hasSetup, errorValue := runSkillSetup(skill, setupOptions, environment, machine, progress)
		if errorValue != nil || hasSetup {
			return errorValue
		}
		fmt.Fprintf(progress, "%s has no setup of its own.\n", skill.setupEntryPath())
	}
	if !declaresPythonRequirements(skill) {
		return nil
	}
	return runSkillRuntimeBootstrap(layout, skill, environment, machine, progress)
}

// runSkillSetup reports whether the skill has a setup at all: a command line
// that answers `setup` with UNKNOWN_COMMAND predates it, and is prepared the
// way any other skill is.
func runSkillSetup(skill bundledSkill, setupOptions []string, environment []string, machine Machine, progress io.Writer) (bool, error) {
	fmt.Fprintf(progress, "Preparing the %s skill with its setup…\n", skill.Name)
	arguments := append(append(append([]string{}, environment...), skill.setupEntryPath(), skillSetupArgument), setupOptions...)
	answer := bytes.Buffer{}
	runError := machine.Stream("env", arguments, Streams{Output: &answer, Errors: progress})
	envelope, parseError := parseSkillEnvelope(answer.Bytes())
	if runError == nil {
		fmt.Fprintln(progress, summaryOf(envelope, parseError, answer.String()))
		return true, nil
	}
	if parseError != nil {
		return true, fmt.Errorf("could not prepare the %s skill: %s %s failed (%v) and printed %q", skill.Name, skill.setupEntryPath(), skillSetupArgument, runError, strings.TrimSpace(answer.String()))
	}
	if envelope.namesIssue(unknownCommandIssueCode) {
		return false, nil
	}
	return true, fmt.Errorf("could not prepare the %s skill: %s", skill.Name, envelope.describe())
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

func (envelope skillEnvelope) namesIssue(code string) bool {
	for _, issue := range envelope.Issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func (envelope skillEnvelope) describe() string {
	lines := []string{envelope.Summary}
	for _, issue := range envelope.Issues {
		lines = append(lines, fmt.Sprintf("  %s: %s %s", issue.Code, issue.Message, issue.Suggestion))
	}
	return strings.Join(lines, "\n")
}

func declaresPythonRequirements(skill bundledSkill) bool {
	if !isRegularFile(skill.scriptPath(skillRuntimeScriptName)) {
		return false
	}
	requirements, errorValue := os.ReadFile(skill.requirementsPath())
	return errorValue == nil && strings.TrimSpace(string(requirements)) != ""
}

// skill_runtime.py makes the environment and re-runs itself in it, then runs
// what it was asked to, which here is nothing.
func runSkillRuntimeBootstrap(layout blueclaw.CompanyHostLayout, skill bundledSkill, environment []string, machine Machine, progress io.Writer) error {
	fmt.Fprintf(progress, "Preparing the %s skill's environment from %s…\n", skill.Name, skill.requirementsPath())
	arguments := []string{skill.scriptPath(skillRuntimeScriptName), "python", "-c", ""}
	if errorValue := machine.Run(layout.PythonPath(), arguments, environment, progress); errorValue != nil {
		return fmt.Errorf("could not prepare the %s skill's environment from %s: %w; the output above names what failed", skill.Name, skill.requirementsPath(), errorValue)
	}
	return nil
}

func isRegularFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.Mode().IsRegular()
}

func isExecutableFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.Mode().IsRegular() && information.Mode().Perm()&0o111 != 0
}
