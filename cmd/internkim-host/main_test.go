package main

import (
	"testing"
)

func TestTheConnectionFileIsFoundWhereverItSitsAmongTheOptions(t *testing.T) {
	expected := installArguments{
		ConnectionPath:     "internkim-host.json",
		StateDirectoryPath: "/state",
		ModelKeyPath:       "/key",
	}
	orders := [][]string{
		{"internkim-host.json", "--state-directory", "/state", "--model-key-file", "/key"},
		{"--state-directory", "/state", "--model-key-file", "/key", "internkim-host.json"},
		{"--state-directory=/state", "internkim-host.json", "--model-key-file=/key"},
	}
	for _, arguments := range orders {
		parsed, errorValue := parseInstallArguments(arguments)
		if errorValue != nil {
			t.Fatalf("%v: %v", arguments, errorValue)
		}
		if parsed != expected {
			t.Fatalf("%v parsed as %+v", arguments, parsed)
		}
	}
}

func TestInstallRefusesAnAmbiguousOrMissingConnectionFile(t *testing.T) {
	for _, arguments := range [][]string{{}, {"--state-directory", "/state"}, {"one.json", "two.json"}} {
		if _, errorValue := parseInstallArguments(arguments); errorValue == nil {
			t.Fatalf("%v was accepted", arguments)
		}
	}
}

func TestReadModelKeyFileIsEmptyWhenNoFileWasNamed(t *testing.T) {
	value, errorValue := readModelKeyFile("")
	if errorValue != nil || value != "" {
		t.Fatalf("readModelKeyFile(\"\") = %q, %v", value, errorValue)
	}
}
