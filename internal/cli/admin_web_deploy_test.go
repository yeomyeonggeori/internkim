package cli

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestAdminWebDeployTargetsProductionPagesBranch(t *testing.T) {
	document, errorValue := os.ReadFile("setup_flow.go")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedCommand := `"bunx", "wrangler", "pages", "deploy", ".svelte-kit/cloudflare", "--project-name", "internkim", "--branch", "main"`
	if !strings.Contains(string(document), expectedCommand) {
		t.Fatalf("admin web deploy must target the production Pages branch")
	}
}

type roundTripFunc func(request *http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}
