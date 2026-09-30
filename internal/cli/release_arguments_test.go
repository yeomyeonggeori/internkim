package cli

import (
	"strings"
	"testing"
)

func TestReleaseRefusesAFlagItsSubcommandDoesNotTake(t *testing.T) {
	repositories, _ := findReleaseSubcommand("repositories")
	errorValue := checkReleaseArguments(repositories, []string{"--chann", "testing"})
	if errorValue == nil {
		t.Fatal("a misspelled --channel was accepted, so release repositories would publish to its default channel")
	}
	if !strings.Contains(errorValue.Error(), "--chann") || !strings.Contains(errorValue.Error(), "--channel") {
		t.Fatalf("the refusal should name the flag given and the flags taken: %v", errorValue)
	}
}

func TestReleaseAcceptsItsFlagsInBothSpellings(t *testing.T) {
	repositories, _ := findReleaseSubcommand("repositories")
	for _, arguments := range [][]string{
		{"--channel", "testing"},
		{"--channel=testing", "--output", "/tmp/repository"},
		{},
	} {
		if errorValue := checkReleaseArguments(repositories, arguments); errorValue != nil {
			t.Fatalf("%v was refused: %v", arguments, errorValue)
		}
	}
}

func TestReleaseRefusesAFlagWithNoValue(t *testing.T) {
	packages, _ := findReleaseSubcommand("packages")
	if checkReleaseArguments(packages, []string{"--architecture"}) == nil {
		t.Fatal("--architecture with nothing after it was accepted")
	}
}

func TestReleaseRefusesAPositionalArgument(t *testing.T) {
	packages, _ := findReleaseSubcommand("packages")
	if checkReleaseArguments(packages, []string{"arm64"}) == nil {
		t.Fatal("a bare word was accepted and would have been ignored")
	}
}
