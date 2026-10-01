package admind

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/llmbackend"
)

func TestLLMFailureEvidenceRequiresAdminAndPreservesDocument(t *testing.T) {
	workspacePath := t.TempDir()
	_, capture := llmbackend.NewExchangeCapture(nil)
	identifier, errorValue := llmbackend.WriteFailureEvidence(workspacePath, map[string]string{"prompt": "private test request"}, capture)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{AdminEmailPath: writeTestFile(t, "admin@example.com"), BlueclawWorkspacePath: workspacePath})
	for _, isAdmin := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodGet, "/admin/api/diagnostics/llm-failure?id="+identifier, nil)
		request.RemoteAddr = "198.51.100.10:443"
		if isAdmin {
			request.Header.Set("Cf-Access-Authenticated-User-Email", "admin@example.com")
		}
		response := httptest.NewRecorder()
		service.router().ServeHTTP(response, request)
		if !isAdmin {
			if response.Code != http.StatusForbidden || strings.Contains(response.Body.String(), "private test request") {
				t.Fatalf("unauthorized evidence response: %d %s", response.Code, response.Body.String())
			}
			continue
		}
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "private test request") || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("authorized evidence response: %d %s", response.Code, response.Body.String())
		}
	}
	information, errorValue := os.Stat(filepath.Join(llmbackend.FailureEvidenceDirectory(workspacePath), identifier+".json"))
	if errorValue != nil || information.Mode().Perm() != 0600 {
		t.Fatalf("evidence permissions: %v %v", information, errorValue)
	}
}

func TestLLMFailureEvidenceRejectsInvalidIdentifiers(t *testing.T) {
	workspacePath := t.TempDir()
	for _, identifier := range []string{"", "../secret", strings.Repeat("a", 31), strings.Repeat("z", 32)} {
		if _, errorValue := llmbackend.ReadFailureEvidence(workspacePath, identifier); errorValue == nil {
			t.Fatalf("accepted invalid identifier %q", identifier)
		}
	}
}
