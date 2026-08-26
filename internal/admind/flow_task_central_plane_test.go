package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestATaskWrittenWithoutARequesterStaysOnTheDevice(t *testing.T) {
	service := &Service{}
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", nil)

	if _, answered, _ := service.saveCentralFlowTask(request, flowTask{Content: "결산"}, nil); answered {
		t.Fatal("the company writes as the person who asked, so a request naming nobody cannot ask it")
	}
	if answered, _ := service.removeCentralFlowTask(request, "task-1"); answered {
		t.Fatal("the same holds for a delete")
	}
}

// The company mints the identifier. A task it has never seen carries none, and
// sending the device's own would ask the company to change a row nobody has.
func TestOnlyAnIdentifierTheCompanyMintedTravelsBackToIt(t *testing.T) {
	if identity := centralTaskIdentityOf(flowTask{ID: "38df3c78-19d9-4b83-995c-eca2b13c44f6"}); identity != "38df3c78-19d9-4b83-995c-eca2b13c44f6" {
		t.Fatalf("identity = %q", identity)
	}
	for _, deviceIdentity := range []string{"", "tool-c808a15bfe125121d09e96cb5aada0b1", "lee-1", "person-sample"} {
		if identity := centralTaskIdentityOf(flowTask{ID: deviceIdentity}); identity != "" {
			t.Fatalf("device identity %q travelled to the company as %q", deviceIdentity, identity)
		}
	}
}
