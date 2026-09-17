package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
)

func TestTaskDefinitionsAnswerTheCompanyVocabulary(t *testing.T) {
	service := NewService(Configuration{TaskDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	company := startCompanyHoldingATaskVocabulary(t)
	useCompanyForTest(service, company.URL)
	ctx := withTaskActor(context.Background(), "someone@example.com")

	definitions, errorValue := service.readTaskDefinitions(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(definitions.Categories, []string{"신사업"}) {
		t.Fatalf("businesses = %v, want the company's own", definitions.Categories)
	}
	if !slices.Equal(definitions.Types, []string{"협상"}) {
		t.Fatalf("types = %v, want the company's own", definitions.Types)
	}
	if len(definitions.Sizes) == 0 {
		t.Fatal("the size rubric is a built-in vocabulary and must still be answered")
	}
}

func TestTaskDefinitionsAnswerNothingWhenNoCompanyIsNamed(t *testing.T) {
	service := NewService(Configuration{TaskDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})

	if _, errorValue := service.readTaskDefinitions(context.Background()); errorValue == nil {
		t.Fatal("a device that names no company has no vocabulary of its own to answer with")
	}
}

func startCompanyHoldingATaskVocabulary(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/session" {
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
			return
		}
		if request.URL.Path != "/api/v1/tools/task_list/invoke" {
			responseWriter.WriteHeader(http.StatusNotFound)
			return
		}
		responseWriter.Write([]byte(`{"result":{"count":0,"tasks":[],` +
			`"registeredLabels":{"businesses":[{"name":"신사업","color":"#2563eb"}],"types":[{"name":"협상"}],"sizes":["XS"],"statuses":["planned"]}}}`))
	}))
	t.Cleanup(server.Close)
	return server
}
