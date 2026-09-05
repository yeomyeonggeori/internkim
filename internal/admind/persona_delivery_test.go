package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonaSyncPublishesCanonicalDocumentsToBlueclaw(t *testing.T) {
	service := personaTestService(t)
	var received agentPersonaDocuments
	var receivedRoute string
	backend := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		receivedRoute = request.Method + " " + request.URL.Path
		if errorValue := json.NewDecoder(request.Body).Decode(&received); errorValue != nil {
			http.Error(response, errorValue.Error(), http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(response).Encode(received)
	}))
	defer backend.Close()
	service.Configuration.BlueclawBaseURL = backend.URL
	if errorValue := service.syncAgentPersona(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if receivedRoute != "PUT /admin/api/persona/agent" || received.Identity.Handle != agentHandle || len(received.Soul.Values) == 0 {
		t.Fatalf("wrong publication: %s %+v", receivedRoute, received)
	}
	for _, name := range []string{identityFileName, soulFileName} {
		if _, errorValue := os.Stat(filepath.Join(service.Configuration.BlueclawWorkspacePath, name)); !os.IsNotExist(errorValue) {
			t.Fatalf("persona was written to the host staging workspace: %s", name)
		}
	}
}

func TestSoulPublicationFailureIsVisibleAndKeepsTheConfiguredDocument(t *testing.T) {
	for _, status := range []int{http.StatusServiceUnavailable, http.StatusOK} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			service := personaTestService(t)
			backend := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				response.WriteHeader(status)
				_, _ = response.Write([]byte(`{}`))
			}))
			defer backend.Close()
			service.Configuration.BlueclawBaseURL = backend.URL
			response := httptest.NewRecorder()
			service.updateSoul(response, httptest.NewRequest(http.MethodPut, "/soul", strings.NewReader(`{"schemaVersion":1,"values":["Preserve evidence."]}`)))
			if response.Code != http.StatusBadGateway {
				t.Fatalf("unconfirmed publication reported success: %d %s", response.Code, response.Body.String())
			}
			response = httptest.NewRecorder()
			service.writeSoul(response, httptest.NewRequest(http.MethodGet, "/soul", nil))
			if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Preserve evidence.") {
				t.Fatalf("configured document became unavailable: %d %s", response.Code, response.Body.String())
			}
		})
	}
}
