package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/box"
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

func TestOutputAnswersWithWhatTheCommandPrintedAndNotWhatItComplained(t *testing.T) {
	script := `echo 'LOG:  42501: could not change directory to "/home/sample": Permission denied' >&2; echo 15`
	answer, errorValue := thisComputer{}.Output("sh", []string{"-c", script})
	if errorValue != nil {
		t.Fatalf("the command failed: %v", errorValue)
	}
	if answer != "15\n" {
		t.Fatalf("the answer was %q", answer)
	}
}

func TestOutputCarriesTheComplaintOfACommandThatFailed(t *testing.T) {
	_, errorValue := thisComputer{}.Output("sh", []string{"-c", "echo 'no such cluster' >&2; exit 3"})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "no such cluster") {
		t.Fatalf("the failure was %v", errorValue)
	}
}

func TestWifiSetupWiresGetOnlineChangeWifiAndScanWifiTogether(t *testing.T) {
	without := boxDaemon("https://example.com")
	if without.GetOnline != nil || without.ChangeWifi != nil || without.ScanWifi != nil {
		t.Fatalf("a daemon built without the flag has wifi hooks: %+v", without)
	}
	with := withWifiSetup(box.Daemon{}, "https://example.com")
	if with.GetOnline == nil || with.ChangeWifi == nil || with.ScanWifi == nil {
		t.Fatalf("a daemon built with the flag lacks a wifi hook: GetOnline=%t ChangeWifi=%t ScanWifi=%t", with.GetOnline != nil, with.ChangeWifi != nil, with.ScanWifi != nil)
	}
}

func TestReachesURLTreatsAnyHTTPResponseAsReachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	address := server.URL
	if !reachesURL(context.Background(), address) {
		t.Fatal("a server answering 503 was judged unreachable")
	}
	server.Close()
	if reachesURL(context.Background(), address) {
		t.Fatal("a closed server was judged reachable")
	}
}
