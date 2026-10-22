package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
)

func TestTheConnectionFileIsFoundWhereverItSitsAmongTheOptions(t *testing.T) {
	expected := installArguments{
		ConnectionPath: "internkim-host.json",
		ModelKeyPath:   "/key",
	}
	orders := [][]string{
		{"internkim-host.json", "--model-key-file", "/key"},
		{"--model-key-file", "/key", "internkim-host.json"},
		{"internkim-host.json", "--model-key-file=/key"},
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
	for _, arguments := range [][]string{{}, {"--model-key-file", "/key"}, {"one.json", "two.json"}} {
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

func TestRunCarriesTheComplaintOfACommandThatFailedEvenWhenNothingReadsItsOutput(t *testing.T) {
	script := "echo 'Failed to enable unit: Unit file internkim-embedding.service does not exist.' >&2; exit 1"
	errorValue := thisComputer{}.Run("sh", []string{"-c", script}, nil, io.Discard)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "internkim-embedding.service does not exist") {
		t.Fatalf("the failure was %v", errorValue)
	}
	var exitError *exec.ExitError
	if !errors.As(errorValue, &exitError) || exitError.ExitCode() != 1 {
		t.Fatalf("the failure no longer carries the exit status: %v", errorValue)
	}
}

func TestRunStillStreamsWhatTheCommandSays(t *testing.T) {
	var output bytes.Buffer
	if errorValue := (thisComputer{}).Run("sh", []string{"-c", "echo progress; echo warning >&2"}, nil, &output); errorValue != nil {
		t.Fatalf("the command failed: %v", errorValue)
	}
	if !strings.Contains(output.String(), "progress") || !strings.Contains(output.String(), "warning") {
		t.Fatalf("the output was %q", output.String())
	}
}

func TestWifiSetupWiresGetOnlineChangeWifiAndScanWifiTogether(t *testing.T) {
	without := boxDaemon("https://example.com")
	if without.GetOnline != nil || without.ChangeWifi != nil || without.ScanWifi != nil {
		t.Fatalf("a daemon built without the flag has wifi hooks: %+v", without)
	}
	with := withWifiSetup(box.Daemon{}, "https://example.com", time.Time{})
	if with.GetOnline == nil || with.ChangeWifi == nil || with.ScanWifi == nil {
		t.Fatalf("a daemon built with the flag lacks a wifi hook: GetOnline=%t ChangeWifi=%t ScanWifi=%t", with.GetOnline != nil, with.ChangeWifi != nil, with.ScanWifi != nil)
	}
}

func TestOnlyAComputerAskedToIsRenamedAfterItsKey(t *testing.T) {
	if without := boxDaemon("https://example.com"); without.NameHost != nil {
		t.Fatal("a daemon built without --host-name-from-company would rename this computer")
	}
	if with := withHostNameFromCompany(box.Daemon{}); with.NameHost == nil {
		t.Fatal("a daemon built with --host-name-from-company does not rename this computer")
	}
	if without := boxDaemon("https://example.com"); without.SetAdminPassword != nil {
		t.Fatal("a daemon built without --admin-account would set an account's password")
	}
	if with := withAdminPasswordFor(box.Daemon{}, "admin"); with.SetAdminPassword == nil || with.LockAdminPassword == nil {
		t.Fatal("a daemon built with --admin-account does not set and lock that account's password")
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

func TestABoxInstalledFromAConnectionAnnouncesToTheAppThatIssuedIt(t *testing.T) {
	connectionPath := filepath.Join(t.TempDir(), "connection.json")
	document := `{"schemaVersion":1,"appURL":"https://company.example.com",` +
		`"company":{"id":"00000000-0000-4000-8000-000000000001","name":"Example Co","slug":"example"},` +
		`"centralPlane":{"projectURL":"https://project.supabase.co","publishableKey":"publishable"},` +
		`"gatewayURL":"wss://gateway.example.com","agentKey":"` + strings.Repeat("a", 64) + `"}`
	if errorValue := os.WriteFile(connectionPath, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	flags := flag.NewFlagSet("box", flag.ContinueOnError)
	appURL := flags.String("app-url", "https://intern.example.test", "")
	if errorValue := flags.Parse(nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	if announced := announcedAppURL(flags, *appURL, t.TempDir(), connectionPath); announced != "https://company.example.com" {
		t.Errorf("a box with no remembered address announced to %s instead of the app its connection names", announced)
	}
	if announced := announcedAppURL(flags, *appURL, t.TempDir(), filepath.Join(t.TempDir(), "absent.json")); announced != "https://intern.example.test" {
		t.Errorf("a box with no connection announced to %s instead of the default", announced)
	}
}
