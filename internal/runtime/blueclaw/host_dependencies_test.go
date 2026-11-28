package blueclaw_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const repositoryRootFromHere = "../../.."

func TestThePackageAsksTheDistributionOnlyForWhatItDoesNotCarry(t *testing.T) {
	depends := strings.Join(blueclaw.HostPackageDependsFor(blueclaw.PackageManagerApt), ", ")
	for _, dependency := range blueclaw.HostDependencies() {
		if dependency.DebianPackage == "" || dependency.ArrivesAsPayload {
			continue
		}
		isNamed := strings.Contains(depends, dependency.DebianPackage)
		if dependency.WhatThePackageCarriesInstead != "" && isNamed {
			t.Errorf("the package carries %s and still asks the distribution for it", dependency.DebianPackage)
		}
	}
	for _, manager := range blueclaw.PackageManagers() {
		for _, name := range blueclaw.HostPackageDependsFor(manager) {
			if strings.Contains(name, "postgresql") {
				t.Errorf("%s is asked for %s, and the package carries its own PostgreSQL", manager, name)
			}
		}
	}
	if slices.Contains(blueclaw.HostHomebrewDependencies(), "postgresql@17") {
		t.Error("the formula depends on postgresql@17, and the keg carries its own PostgreSQL")
	}
}

func TestNoManagerIsAskedForPgvector(t *testing.T) {
	asked := map[string][]string{"brew": blueclaw.HostHomebrewDependencies()}
	for _, manager := range []blueclaw.PackageManager{blueclaw.PackageManagerApt, blueclaw.PackageManagerDnf, blueclaw.PackageManagerPacman} {
		asked[string(manager)] = blueclaw.HostPackageDependsFor(manager)
	}
	for manager, depends := range asked {
		if slices.ContainsFunc(depends, func(name string) bool { return strings.Contains(name, "pgvector") }) {
			t.Errorf("%s is asked for pgvector, and nothing in the host's databases uses it: %v", manager, depends)
		}
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
