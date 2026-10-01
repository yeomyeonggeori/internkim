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
	if !strings.Contains(depends, "postgresql, ") {
		t.Errorf("the package must ask for the distribution's own PostgreSQL under one unversioned name, got %q", depends)
	}
}

// The memory store searches by embedding only where the agent's database has
// pgvector, and keeps answering by words without it, so nothing fails on a
// host that lacks it. Debian names pgvector once per PostgreSQL major, and apt
// takes the first alternative its archive carries, so the newest comes first
// and the oldest is the oldest the schema runs on.
func TestEveryManagerAsksForTheVectorExtensionTheMemoryStoreSearchesWith(t *testing.T) {
	expected := map[blueclaw.PackageManager]string{
		blueclaw.PackageManagerApt: "postgresql-18-pgvector | postgresql-17-pgvector | postgresql-16-pgvector | " +
			"postgresql-15-pgvector | postgresql-14-pgvector",
		blueclaw.PackageManagerDnf:    "pgvector",
		blueclaw.PackageManagerPacman: "pgvector",
	}
	for manager, names := range expected {
		depends := blueclaw.HostPackageDependsFor(manager)
		if !slices.Contains(depends, names) {
			t.Errorf("%s's dependency list does not ask for %q, so a host installs without vector search: %v", manager, names, depends)
		}
	}
	if !slices.Contains(blueclaw.HostHomebrewDependencies(), "pgvector") {
		t.Errorf("the formula does not depend on pgvector: %v", blueclaw.HostHomebrewDependencies())
	}
}

func TestTheVectorExtensionANamedMajorLoadsIsThatMajorsPackage(t *testing.T) {
	dependency := blueclaw.VectorExtensionDependencyFor(16)
	if names := dependency.PackagesFor(blueclaw.PackageManagerApt); !slices.Equal(names, []string{"postgresql-16-pgvector"}) {
		t.Errorf("PostgreSQL 16 on apt is told to install %v", names)
	}
	if names := dependency.PackagesFor(blueclaw.PackageManagerDnf); !slices.Equal(names, []string{"pgvector"}) {
		t.Errorf("PostgreSQL 16 on dnf is told to install %v", names)
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
