package blueclaw_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestEveryManagerNamesEachDependencyOrSaysWhatBringsItInstead(t *testing.T) {
	for _, manager := range blueclaw.PackageManagers() {
		for _, dependency := range blueclaw.HostDependencies() {
			if !dependency.IsNamedIn(manager) {
				continue
			}
			if len(dependency.PackagesFor(manager)) == 0 || dependency.PackagesFor(manager)[0] == "" {
				t.Errorf("%s names no package for %s and says nothing brings it instead, so a native install on it "+
					"would be a host missing one of its dependencies", dependency.DebianPackage, manager)
			}
		}
	}
}

func TestEveryReasonWhatBringsItInsteadNamesAManagerThatExists(t *testing.T) {
	known := map[blueclaw.PackageManager]bool{}
	for _, manager := range blueclaw.PackageManagers() {
		known[manager] = true
	}
	for _, dependency := range blueclaw.HostDependencies() {
		for manager := range dependency.WhatBringsItInstead {
			if !known[manager] {
				t.Errorf("%s says something brings it instead on %q, which is not a manager", dependency.DebianPackage, manager)
			}
		}
	}
}

func TestPacmanHasNoAlternativesSoItIsGivenOneNameEach(t *testing.T) {
	for _, dependency := range blueclaw.HostDependencies() {
		if names := dependency.PackagesFor(blueclaw.PackageManagerPacman); len(names) > 1 {
			t.Errorf("%s gives pacman %v, and pacman has no syntax for alternatives", dependency.DebianPackage, names)
		}
	}
}

func TestEveryFormatStatesTheSameGlibcFloorFromOneConstant(t *testing.T) {
	for _, manager := range blueclaw.PackageManagers() {
		depends := strings.Join(blueclaw.HostPackageDependsFor(manager), ", ")
		if !strings.Contains(depends, blueclaw.HostGlibcMinimum) {
			t.Errorf("%s's dependency list does not name glibc %s: %s", manager, blueclaw.HostGlibcMinimum, depends)
		}
	}
}

// install.sh is shell and cannot read this list, so it carries its own copy of
// the managers in the order it asks for them. This reads the script and fails
// when the copy drifts.
func TestInstallScriptAsksForTheDeclaredManagersInOrder(t *testing.T) {
	document, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "web", "static", "install.sh"))
	if readError != nil {
		t.Fatal(readError)
	}
	candidates := regexp.MustCompile(`for candidate in ([^;]+); do`).FindStringSubmatch(string(document))
	if candidates == nil {
		t.Fatal("install.sh no longer looks for a package manager with `for candidate in ...; do`")
	}
	declared := []string{}
	for _, manager := range blueclaw.PackageManagers() {
		declared = append(declared, string(manager))
	}
	if strings.Join(strings.Fields(candidates[1]), " ") != strings.Join(declared, " ") {
		t.Errorf("install.sh looks for %q and the declaration names %q", candidates[1], strings.Join(declared, " "))
	}
}
