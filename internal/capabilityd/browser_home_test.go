package capabilityd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/browserhome"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func serviceWithHomes(t *testing.T, gateway http.HandlerFunc) Service {
	t.Helper()
	directory := t.TempDir()
	server := httptest.NewServer(gateway)
	t.Cleanup(server.Close)
	homesPath := filepath.Join(directory, "browser-homes.json")
	sessionPath := filepath.Join(directory, "agent-key")
	if errorValue := os.WriteFile(sessionPath, []byte("host.session.token\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	homes := browserhome.Homes{ThisBox: "head-office", GatewayURL: server.URL, CompanyID: "company-1", Members: map[string]string{"sample@example.com": "annex"}}
	if errorValue := browserhome.Write(homesPath, homes); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{Configuration: Configuration{BrowserHomesPath: homesPath, HostSessionPath: sessionPath}}
}

func browserOpenFor(email string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: "browser_open",
		Input:    []byte(`{"url":"https://example.com"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: email},
	}
}

func TestABrowserCallForAPersonPlacedOnAnotherBoxIsAnsweredByThatBox(t *testing.T) {
	service := serviceWithHomes(t, func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer host.session.token" {
			t.Errorf("the hand-off carried %q", request.Header.Get("Authorization"))
		}
		writer.Write([]byte(`{"requestID":"x","status":200,"body":{"toolName":"browser_open","status":"ok","content":"opened on the annex"}}`))
	})

	response, errorValue := service.invokeDeviceBrowserTool(context.Background(), browserOpenFor("sample@example.com"))

	if errorValue != nil || response.Content != "opened on the annex" {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestABrowserCallRunsHereWhenItsBoxIsAwayOrItIsPlacedHere(t *testing.T) {
	handedOn := 0
	service := serviceWithHomes(t, func(writer http.ResponseWriter, _ *http.Request) {
		handedOn++
		http.Error(writer, "not connected", http.StatusServiceUnavailable)
	})

	for _, email := range []string{"sample@example.com", "someone-else@example.com"} {
		_, errorValue := service.invokeDeviceBrowserTool(context.Background(), browserOpenFor(email))
		if errorValue == nil || !strings.Contains(errorValue.Error(), "capabilityd runs no device browsers") {
			t.Fatalf("%s was not run on this box: %v", email, errorValue)
		}
	}
	if handedOn != 1 {
		t.Fatalf("handed on %d times; only the person placed elsewhere is tried there", handedOn)
	}
}
