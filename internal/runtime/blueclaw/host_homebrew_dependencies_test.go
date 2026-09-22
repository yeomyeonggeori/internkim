package blueclaw

import (
	"strings"
	"testing"
)

// A formula cannot depend on a cask. Homebrew's DependencyCollector raises
// "Unsupported special dependency" on anything outside :arch, :linux, :macos,
// :maximum_macos and :xcode, and `cask:` is a key of the cask DSL's own
// depends_on. A cask name that reached depends_on would be read as a formula
// name, and `brew install internkim` would fail on a formula nobody publishes.
func TestNoCaskIsRenderedAsAFormulaDependency(t *testing.T) {
	caskNames := map[string]bool{}
	for _, cask := range HostHomebrewCasksAPersonMustInstall() {
		caskNames[cask.Name] = true
	}
	if len(caskNames) == 0 {
		t.Fatal("no dependency names a cask, so this test is watching nothing")
	}
	for _, formula := range HostHomebrewDependencies() {
		if caskNames[formula] {
			t.Fatalf("%s is a cask and is being rendered as a depends_on line", formula)
		}
	}
}

// A caveat that says "brew install --cask chromium" on a Mac where that cask is
// disabled sends a person at a command that fails and tells them nothing. A
// disabled cask has to carry both the reason and what to do instead.
func TestADisabledCaskSaysWhyAndWhatToInstallInstead(t *testing.T) {
	checked := 0
	for _, cask := range HostHomebrewCasksAPersonMustInstall() {
		if cask.WhatItIsFor == "" {
			t.Fatalf("%s is a cask a person has to install by hand and nothing says what it is for", cask.Name)
		}
		if !cask.IsDisabledUpstream {
			continue
		}
		checked++
		if cask.WhyItIsDisabled == "" {
			t.Fatalf("%s is disabled upstream and the declaration does not say why", cask.Name)
		}
		if cask.InsteadInstall == "" || cask.InsteadInstall == cask.Name {
			t.Fatalf("%s is disabled upstream and the declaration offers nothing that works instead", cask.Name)
		}
	}
	if checked == 0 {
		t.Fatal("no cask is marked disabled upstream, and chromium's is; this test is watching the wrong field")
	}
}

// Every dependency has to be answered on a Mac by exactly one thing that can be
// pointed at: a formula the package depends on, a cask a person installs, a path
// the Mac already has, or a sentence saying the Mac supplies it. An empty answer
// used to mean both "macOS has it" and "nobody looked", and that is the state
// this test exists to make impossible.
func TestEveryDependencyHasOneNamedAnswerOnAMac(t *testing.T) {
	for _, dependency := range HostDependencies() {
		answers := []string{}
		if dependency.HomebrewFormula != "" {
			answers = append(answers, "the formula depends on "+dependency.HomebrewFormula)
		}
		if dependency.HomebrewCask.IsDeclared() {
			answers = append(answers, "the caveats name the "+dependency.HomebrewCask.Name+" cask")
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

// The declaration's own account of the Mac has to match what the skills would
// actually find. A candidate path that names a directory rather than the program
// inside an application bundle would pass a check and then fail to execute.
func TestTheBrowserCandidatesNameProgramsRatherThanBundles(t *testing.T) {
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
		t.Fatal("no dependency names where it is on a Mac, and two of them are only findable that way")
	}
}

func dependencyDescription(dependency HostDependency) string {
	if dependency.DebianPackage != "" {
		return dependency.DebianPackage
	}
	return strings.Join(dependency.ProgramsTheHostRuns, ", ")
}
