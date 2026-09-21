package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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

func pipeHolding(t *testing.T, content string) *os.File {
	t.Helper()
	reader, writer, errorValue := os.Pipe()
	if errorValue != nil {
		t.Fatalf("open a pipe: %v", errorValue)
	}
	go func() {
		writer.WriteString(content)
		writer.Close()
	}()
	t.Cleanup(func() { reader.Close() })
	return reader
}

func TestTheModelKeyIsReadFromStandardInputWhenThereIsNoTerminal(t *testing.T) {
	var prompt bytes.Buffer
	key, errorValue := readModelKey(pipeHolding(t, "sk-or-piped\n"), &prompt)
	if errorValue != nil {
		t.Fatalf("read the key from a pipe: %v", errorValue)
	}
	if strings.TrimSpace(key) != "sk-or-piped" {
		t.Fatalf("read %q from the pipe", key)
	}
}

func TestAnEmptyStandardInputNamesTheOptionThatSuppliesTheKey(t *testing.T) {
	var prompt bytes.Buffer
	_, errorValue := readModelKey(pipeHolding(t, ""), &prompt)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "--model-key-file") {
		t.Fatalf("an empty standard input returned %v", errorValue)
	}
}

func TestAModelKeyFileThatHoldsNothingIsRefusedRatherThanIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "model-key")
	if errorValue := os.WriteFile(path, []byte("   \n"), 0o600); errorValue != nil {
		t.Fatalf("write the empty key file: %v", errorValue)
	}
	_, errorValue := readModelKeyFile(path)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "holds no OpenRouter API key") {
		t.Fatalf("an empty key file returned %v", errorValue)
	}
}
