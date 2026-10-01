package cli

import (
	"strings"
	"testing"
)

func TestReleaseRefusesAFlagItsSubcommandDoesNotTake(t *testing.T) {
	host, _ := findReleaseSubcommand("host")
	errorValue := checkReleaseArguments(host, []string{"--chann", "testing"})
	if errorValue == nil {
		t.Fatal("a misspelled --channel was accepted")
	}
	if !strings.Contains(errorValue.Error(), "--chann") || !strings.Contains(errorValue.Error(), "--channel") {
		t.Fatalf("the refusal should name the flag given and the flags taken: %v", errorValue)
	}
}

func TestReleaseAcceptsItsFlagsInBothSpellings(t *testing.T) {
	packages, _ := findReleaseSubcommand("packages")
	for _, arguments := range [][]string{
		{"--architecture", "arm64"},
		{"--architecture=arm64", "--out", "/tmp/release"},
		{},
	} {
		if errorValue := checkReleaseArguments(packages, arguments); errorValue != nil {
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
