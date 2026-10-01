package centralplane

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol/jsonschema"
)

func TestTaskLabelsReadAnAnswerTheCatalogPromises(t *testing.T) {
	const result = `{"tasks":[],"count":0,"unfinishedCount":0,"scope":"self",` +
		`"registeredLabels":{"businesses":[{"name":"신사업","color":"#2563eb"}],` +
		`"types":[{"name":"협상"}],"sizes":["XS"],"statuses":["planned"]}}`

	descriptor := capabilityprotocol.MustGeneratedToolDescriptors("task_list")[0]
	check, errorValue := jsonschema.ValidateResult(descriptor.ResultContract.Schema, []byte(result))
	if errorValue != nil {
		t.Fatalf("the answer this reads is not one task_list gives: %v", errorValue)
	}
	if len(check.UnknownFields) > 0 {
		t.Fatalf("the answer this reads carries fields task_list never gives: %v", check.UnknownFields)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/agent/session":
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
		case "/api/v1/tools/task_list/invoke":
			writer.Write([]byte(`{"result":` + result + `}`))
		default:
			http.Error(writer, "unexpected "+request.URL.Path, http.StatusTeapot)
		}
	}))
	t.Cleanup(server.Close)
	client := New(Settings{AppURL: server.URL, HostCredential: func() string { return "agent-key" }, ProjectURL: server.URL, PublishableKey: "publishable-key"})

	labels, errorValue := client.TaskLabels(context.Background(), "member1@example.com")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	want := TaskLabels{Businesses: []string{"신사업"}, Types: []string{"협상"}}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %#v, want %#v", labels, want)
	}
}
