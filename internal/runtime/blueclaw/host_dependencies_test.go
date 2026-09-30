package blueclaw_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const repositoryRootFromHere = "../../.."

func logicalLinesOf(document string) []string {
	joined := []string{}
	pending := ""
	for _, line := range strings.Split(document, "\n") {
		continues := strings.HasSuffix(line, "\\")
		if continues {
			line = strings.TrimSuffix(line, "\\")
		}
		if pending == "" {
			pending = line
		} else {
			pending += " " + strings.TrimSpace(line)
		}
		if continues {
			continue
		}
		if strings.TrimSpace(pending) != "" {
			joined = append(joined, strings.TrimSpace(pending))
		}
		pending = ""
	}
	return joined
}

func shellCommandsIn(text string) []string {
	separated := text
	for _, separator := range []string{"&&", "||", ";", "|"} {
		separated = strings.ReplaceAll(separated, separator, "\n")
	}
	return strings.Split(separated, "\n")
}

func operandsOf(words []string) []string {
	optionsCarryingASeparateValue := map[string]bool{"-o": true, "-t": true, "-c": true}
	operands := []string{}
	for index := 0; index < len(words); index++ {
		if optionsCarryingASeparateValue[words[index]] {
			index++
			continue
		}
		if strings.HasPrefix(words[index], "-") {
			continue
		}
		operands = append(operands, words[index])
	}
	return operands
}

func packagesTheHostImageInstalls(t *testing.T) []string {
	t.Helper()
	document, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "host", "Dockerfile"))
	if readError != nil {
		t.Fatal(readError)
	}
	installed := []string{}
	foundAnInstall := false
	for _, logicalLine := range logicalLinesOf(string(document)) {
		if !strings.HasPrefix(logicalLine, "RUN ") {
			continue
		}
		for _, command := range shellCommandsIn(strings.TrimPrefix(logicalLine, "RUN ")) {
			words := strings.Fields(command)
			for len(words) > 0 && strings.Contains(words[0], "=") {
				words = words[1:]
			}
			if len(words) < 2 || words[0] != "apt-get" || words[1] != "install" {
				continue
			}
			foundAnInstall = true
			installed = append(installed, operandsOf(words[2:])...)
		}
	}
	if !foundAnInstall {
		t.Fatal("host/Dockerfile runs no apt-get install, so this test reads nothing and would pass on an image that installs anything at all")
	}
	return installed
}

func sorted(names []string) []string {
	copied := append([]string(nil), names...)
	sort.Strings(copied)
	return copied
}

func TestTheHostImageInstallsExactlyTheDeclaredPackages(t *testing.T) {
	installed := sorted(packagesTheHostImageInstalls(t))
	declared := sorted(blueclaw.HostImageDebianPackages())
	if strings.Join(installed, " ") == strings.Join(declared, " ") {
		return
	}
	t.Fatalf("host/Dockerfile installs %v and internal/runtime/blueclaw declares %v; the two are the same list, "+
		"so a package added to one and not the other is a company host that either lacks what its skills need "+
		"or carries what nothing asked for", installed, declared)
}

type aBoxCarrying struct {
	pathDirectory  string
	entrypointPath string
}

func aBoxCarryingOnly(t *testing.T, programs []string) aBoxCarrying {
	t.Helper()
	entrypointPath, absoluteError := filepath.Abs(filepath.Join(repositoryRootFromHere, "host", "entrypoint.sh"))
	if absoluteError != nil {
		t.Fatal(absoluteError)
	}
	box := aBoxCarrying{pathDirectory: t.TempDir(), entrypointPath: entrypointPath}
	for _, program := range programs {
		if writeError := os.WriteFile(filepath.Join(box.pathDirectory, program), []byte("#!/bin/sh\nexit 0\n"), 0o755); writeError != nil {
			t.Fatal(writeError)
		}
	}
	return box
}

func (box aBoxCarrying) withoutIts(t *testing.T, program string, run func()) {
	t.Helper()
	present := filepath.Join(box.pathDirectory, program)
	absent := present + ".taken-away"
	if renameError := os.Rename(present, absent); renameError != nil {
		t.Fatal(renameError)
	}
	defer func() {
		if renameError := os.Rename(absent, present); renameError != nil {
			t.Fatal(renameError)
		}
	}()
	run()
}

func (box aBoxCarrying) whatTheEntrypointSaysIsMissing(t *testing.T) []string {
	t.Helper()
	command := exec.Command("/bin/sh", box.entrypointPath, "--check-programs")
	command.Env = []string{"PATH=" + box.pathDirectory}
	complaint := strings.Builder{}
	command.Stderr = &complaint
	_ = command.Run()
	said := []string{}
	for _, line := range strings.Split(complaint.String(), "\n") {
		if strings.TrimSpace(line) != "" {
			said = append(said, line)
		}
	}
	return said
}

func aComplaintAbout(program string) func(string) bool {
	opening := "[host] this image carries no " + program
	return func(line string) bool {
		if !strings.HasPrefix(line, opening) {
			return false
		}
		rest := line[len(opening):]
		return rest == "" || strings.HasPrefix(rest, ",")
	}
}

func everyProgramTheDeclarationNames() []string {
	return append(blueclaw.ProgramsTheHostEntrypointRuns(), blueclaw.ProgramsTheBundledSkillsRun()...)
}

func TestTheEntrypointChecksForExactlyTheProgramsTheDeclarationNames(t *testing.T) {
	everyProgram := everyProgramTheDeclarationNames()
	box := aBoxCarryingOnly(t, everyProgram)

	for _, line := range box.whatTheEntrypointSaysIsMissing(t) {
		for _, program := range everyProgram {
			if aComplaintAbout(program)(line) {
				t.Errorf("a box carrying every program internal/runtime/blueclaw declares is still told %q, "+
					"so host/entrypoint.sh checks for something the declaration does not name", line)
			}
		}
	}

	for _, program := range everyProgram {
		box.withoutIts(t, program, func() {
			for _, line := range box.whatTheEntrypointSaysIsMissing(t) {
				if aComplaintAbout(program)(line) {
					return
				}
			}
			t.Errorf("host/entrypoint.sh --check-programs says nothing about a box with no %s, so the declaration "+
				"names a program the box is never asked for", program)
		})
	}
}

func TestTheEntrypointNamesTheFilesTheDeclarationDoes(t *testing.T) {
	complaints := aBoxCarryingOnly(t, everyProgramTheDeclarationNames()).whatTheEntrypointSaysIsMissing(t)
	checked := 0
	for _, file := range blueclaw.HostFilesTheBundledSkillsRead() {
		if _, statError := os.Stat(file.Path); statError == nil {
			continue
		}
		checked++
		named := false
		for _, line := range complaints {
			named = named || (strings.Contains(line, file.Path) && strings.Contains(line, file.DebianPackage))
		}
		if !named {
			t.Errorf("a box with no %s is not told about it by host/entrypoint.sh --check-programs, or is told to "+
				"install something other than %s, and a missing font produces a plausible PDF rather than an error",
				file.Path, file.DebianPackage)
		}
	}
	if checked == 0 {
		t.Skip("this machine carries every file the bundled skills read, so their absence cannot be observed here")
	}
}

func TestThePackageDependsOnEverythingTheImageInstallsThatItDoesNotCarry(t *testing.T) {
	depends := blueclaw.HostDebianDependsLine()
	carried := map[string]bool{}
	for _, dependency := range blueclaw.HostDependencies() {
		if dependency.WhatTheDebianPackageCarriesInstead != "" {
			carried[dependency.DebianPackage] = true
		}
	}
	for _, packageName := range blueclaw.HostImageDebianPackages() {
		isNamed := strings.Contains(depends, packageName)
		if carried[packageName] && isNamed {
			t.Errorf("the package carries %s and still asks the distribution for it", packageName)
		}
		if !carried[packageName] && !isNamed {
			t.Errorf("the image installs %s and the package neither depends on it nor carries it, so a native "+
				"install is a host the container path proved it needs more than", packageName)
		}
	}
	if !strings.Contains(depends, "postgresql (>= 14)") {
		t.Errorf("the package must name the oldest PostgreSQL the schema runs on, got %q", depends)
	}
}

func TestEveryHostDependencyIsDeclaredOnceAndNeededBySomething(t *testing.T) {
	seen := map[string]bool{}
	for _, dependency := range blueclaw.HostDependencies() {
		name := dependency.DebianPackage
		if name == "" {
			name = strings.Join(dependency.ProgramsTheHostRuns, " ")
		}
		if seen[name] {
			t.Errorf("%s is declared twice, which is the thing this declaration exists to stop", name)
		}
		seen[name] = true
		if len(dependency.NeededBy) == 0 {
			t.Errorf("%s is declared and nothing needs it, so no consumer derives it", name)
		}
	}
}
