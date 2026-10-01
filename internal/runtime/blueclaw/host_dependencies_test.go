package blueclaw_test

import (
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
	if !strings.Contains(depends, "postgresql") || strings.Contains(depends, "pgvector") {
		t.Errorf("the package must ask for the distribution's own PostgreSQL under one unversioned name and no pgvector, got %q", depends)
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
