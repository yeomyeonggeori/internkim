package cli

import (
	"strings"
	"testing"
)

func TestReleaseRefusesAFlagItsSubcommandDoesNotTake(t *testing.T) {
	apt, _ := findReleaseSubcommand("apt")
	errorValue := checkReleaseArguments(apt, []string{"--suit", "testing"})
	if errorValue == nil {
		t.Fatal("a misspelled --suite was accepted, so apt would publish to its default suite")
	}
	if !strings.Contains(errorValue.Error(), "--suit") || !strings.Contains(errorValue.Error(), "--suite") {
		t.Fatalf("the refusal should name the flag given and the flags taken: %v", errorValue)
	}
}

func TestReleaseAcceptsItsFlagsInBothSpellings(t *testing.T) {
	apt, _ := findReleaseSubcommand("apt")
	for _, arguments := range [][]string{
		{"--suite", "testing"},
		{"--suite=testing", "--output", "/tmp/repository"},
		{},
	} {
		if errorValue := checkReleaseArguments(apt, arguments); errorValue != nil {
			t.Fatalf("%v was refused: %v", arguments, errorValue)
		}
	}
}

func TestReleaseRefusesAFlagWithNoValue(t *testing.T) {
	deb, _ := findReleaseSubcommand("deb")
	if checkReleaseArguments(deb, []string{"--architecture"}) == nil {
		t.Fatal("--architecture with nothing after it was accepted")
	}
}

func TestReleaseRefusesAPositionalArgument(t *testing.T) {
	deb, _ := findReleaseSubcommand("deb")
	if checkReleaseArguments(deb, []string{"arm64"}) == nil {
		t.Fatal("a bare word was accepted and would have been ignored")
	}
}
