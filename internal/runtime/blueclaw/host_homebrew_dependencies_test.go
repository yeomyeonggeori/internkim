package blueclaw

import (
	"strings"
	"testing"
)

// Every dependency has to be answered on a Mac by exactly one thing that can be
// pointed at: a formula the package depends on, a path
// the Mac already has, or a sentence saying the Mac supplies it. An empty answer
// used to mean both "macOS has it" and "nobody looked", and that is the state
// this test exists to make impossible.
func TestEveryDependencyHasOneNamedAnswerOnAMac(t *testing.T) {
	for _, dependency := range HostDependencies() {
		answers := []string{}
		if dependency.HomebrewFormula != "" {
			answers = append(answers, "the formula depends on "+dependency.HomebrewFormula)
		}
		if len(dependency.MacFilePathCandidates) > 0 {
			answers = append(answers, "it is at "+strings.Join(dependency.MacFilePathCandidates, " or "))
		}
		if dependency.WhatAnswersItOnAMac != "" {
			answers = append(answers, dependency.WhatAnswersItOnAMac)
		}
		if len(answers) == 0 {
			t.Fatalf("nothing says how %s is answered on a Mac", dependencyDescription(dependency))
		}
	}
}

// A candidate path that names an application bundle rather than a file inside
// it would pass a check and then fail to be read.
func TestTheMacFileCandidatesNameFilesRatherThanBundles(t *testing.T) {
	checked := 0
	for _, dependency := range HostDependencies() {
		for _, candidate := range dependency.MacFilePathCandidates {
			if !strings.HasPrefix(candidate, "/") {
				t.Fatalf("%s is not an absolute path", candidate)
			}
			if strings.HasSuffix(candidate, ".app") {
				t.Fatalf("%s is an application bundle; the program inside it is what runs", candidate)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no dependency names where it is on a Mac")
	}
}

func dependencyDescription(dependency HostDependency) string {
	if dependency.DebianPackage != "" {
		return dependency.DebianPackage
	}
	return strings.Join(dependency.ProgramsTheHostRuns, ", ")
}
