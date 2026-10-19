package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPersonaSyncPublishesCanonicalDocumentsToBlueclaw(t *testing.T) {
	service := personaTestService(t)
	var received agentPersonaDocuments
	var receivedRoute string
	backend := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		receivedRoute = request.Method + " " + request.URL.Path
		if request.Method == http.MethodGet {
			_ = json.NewEncoder(response).Encode(rawAgentPersonaDocuments{Identity: json.RawMessage(`{"schemaVersion":1,"names":["김인턴"],"handle":"internkim"}`), Soul: json.RawMessage(`{"schemaVersion":1,"values":["Preserve evidence."]}`)})
			return
		}
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
