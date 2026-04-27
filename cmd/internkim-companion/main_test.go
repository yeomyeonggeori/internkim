package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

func TestDefaultCapabilitiesAdvertiseLLMOnlyInDevelopmentMockMode(t *testing.T) {
	withoutMockLLM := defaultCapabilities(true, false)
	withMockLLM := defaultCapabilities(true, true)

	if hasCapability(withoutMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability to be hidden without development mock mode")
	}
	if !hasCapability(withMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability in development mock mode")
	}
	if !hasCapability(withoutMockLLM, "browser.navigate") {
		t.Fatal("expected browser capability to be advertised")
	}
}

func TestMainSubcommandsReturnAfterExecution(t *testing.T) {
	for _, commandName := range []string{"pair", "run", "status"} {
		if !isCompanionSubcommand(commandName) {
			t.Fatalf("expected %s to be a subcommand", commandName)
		}
	}
	if isCompanionSubcommand("serve") {
		t.Fatal("serve should fall through to default server mode")
	}
}

func hasCapability(descriptors []capabilities.Descriptor, name string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == name {
			return true
		}
	}
	return false
}

func TestPairSavesState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_internkim/companion/pair" {
			t.Fatalf("unexpected pair path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"companionID":"companion-1","token":"token-1"}`), nil
	})}

	errorValue := runPair([]string{"--device-url", "https://device.intern.kim", "--code", "ABCD-1234", "--state", statePath, "--local-only", "--dev-mock-llm"}, httpClient)
	if errorValue != nil {
		t.Fatalf("expected pair success: %v", errorValue)
	}
	state, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CompanionID != "companion-1" || state.Token != "token-1" || !state.LocalOnly {
		t.Fatalf("unexpected state: %+v", state)
	}
	if !hasCapability(state.Capabilities, "llm.structured") {
		t.Fatal("expected development LLM capability to be stored")
	}
}

func TestPairAcceptsDeepLinkArgument(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://device.intern.kim/_internkim/companion/pair" {
			t.Fatalf("unexpected pair endpoint: %s", request.URL.String())
		}
		return textResponse(http.StatusOK, `{"companionID":"companion-1","token":"token-1"}`), nil
	})}

	errorValue := runPair([]string{"--state", statePath, "internkim://pair?device_url=https%3A%2F%2Fdevice.intern.kim&code=ABCD-1234"}, httpClient)
	if errorValue != nil {
		t.Fatalf("expected deep link pair success: %v", errorValue)
	}
	state, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.DeviceURL != "https://device.intern.kim" {
		t.Fatalf("unexpected device URL: %s", state.DeviceURL)
	}
}

func TestRunOnceCompletesMockLLMJob(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state := companionState{
		DeviceURL:    "https://device.intern.kim",
		CompanionID:  "companion-1",
		Token:        "token-1",
		LocalOnly:    true,
		Capabilities: defaultCapabilities(true, true),
	}
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenComplete := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","request":{"toolName":"llm.structured","input":{"structuredOutputSchema":{"document":{"required":["reply"]}}}}}`), nil
		case "/_internkim/companion/jobs/job-1/complete":
			seenComplete = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanion([]string{"--state", statePath, "--once", "--dev-mock-llm"}, httpClient)
	if errorValue != nil {
		t.Fatalf("expected run once success: %v", errorValue)
	}
	if !seenComplete {
		t.Fatal("expected companion to complete the job")
	}
}

func TestStatusRequiresPairedState(t *testing.T) {
	errorValue := runStatus([]string{"--state", filepath.Join(t.TempDir(), "missing.json")})
	if errorValue == nil {
		t.Fatal("expected missing state to fail")
	}
}

func TestStatusJSONReportsUnpairedState(t *testing.T) {
	errorValue := runStatus([]string{"--state", filepath.Join(t.TempDir(), "missing.json"), "--json"})
	if errorValue != nil {
		t.Fatalf("expected missing JSON status to succeed: %v", errorValue)
	}
}

func TestCompanionStatusFromState(t *testing.T) {
	state := companionState{
		DeviceURL:   "https://device.intern.kim",
		CompanionID: "companion-1",
		Token:       "token-1",
		LocalOnly:   true,
	}

	document, errorValue := json.Marshal(companionStatusFromState(state))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), `"paired":true`) {
		t.Fatalf("expected paired JSON, got %s", string(document))
	}
}

func textResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
