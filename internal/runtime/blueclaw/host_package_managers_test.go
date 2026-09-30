package blueclaw_test

import (
	"os"
	"path/filepath"
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

func TestEveryManagerHasACandidateForTheMemoryStore(t *testing.T) {
	for _, manager := range blueclaw.PackageManagers() {
		candidates := blueclaw.HostMemoryStorePackageCandidates(manager)
		if len(candidates) == 0 {
			t.Errorf("%s has no pgvector candidate, so install.sh cannot tell whether its repositories carry one", manager)
		}
		for _, candidate := range candidates {
			if !strings.Contains(candidate, "pgvector") {
				t.Errorf("%s names %q as a memory store candidate, which is not pgvector", manager, candidate)
			}
		}
	}
}

func TestInstallScriptCarriesTheNamesTheTableRenders(t *testing.T) {
	script, readError := os.ReadFile(filepath.Join(repositoryRootFromHere, "web", "static", "install.sh"))
	if readError != nil {
		t.Fatal(readError)
	}
	begin := strings.Index(string(script), blueclaw.InstallScriptDeclarationsBegin)
	end := strings.Index(string(script), blueclaw.InstallScriptDeclarationsEnd)
	if begin < 0 || end < begin {
		t.Fatalf("web/static/install.sh has no block between %q and %q", blueclaw.InstallScriptDeclarationsBegin, blueclaw.InstallScriptDeclarationsEnd)
	}
	carried := string(script)[begin : end+len(blueclaw.InstallScriptDeclarationsEnd)]
	if carried != blueclaw.InstallScriptDeclarations() {
		t.Fatalf("install.sh carries a copy of the dependency table that has drifted from it. Replace the block with:\n%s", blueclaw.InstallScriptDeclarations())
	}
}
