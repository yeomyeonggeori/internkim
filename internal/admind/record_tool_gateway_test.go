package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestARecordToolPathNamesItsTool(t *testing.T) {
	for path, expected := range map[string]string{
		"/record/api/tools/leave_list/invoke":   "leave_list",
		"/record/api/tools/leave_decide/invoke": "leave_decide",
		"/record/api/tools/leave_list":          "",
		"/record/api/tools/task_delete/target":  "task_delete",
		"/record/api/tools//invoke":             "",
		"/record/api/tools/leave_list/read":     "",
		"/task/api/state":                       "",
	} {
		if named, _ := recordToolCallOf(path); named != expected {
			t.Errorf("%s named %q, wanted %q", path, named, expected)
		}
	}
}

// The requester header is honoured only on the socket, so a call that arrives
// anywhere else has asserted nobody and the record must not be asked on
// anybody's behalf.
func TestARecordToolRefusesACallThatAssertsNobody(t *testing.T) {
	service := &Service{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/record/api/tools/leave_list/invoke", strings.NewReader(`{}`))

	service.handleRecordTool(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("a call asserting nobody answered %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "asserted nobody") {
		t.Fatalf("the refusal does not say why: %s", recorder.Body.String())
	}
}

func TestARecordToolIsInvokedWithPost(t *testing.T) {
	service := &Service{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/record/api/tools/leave_list/invoke", nil)

	service.handleRecordTool(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("a GET answered %d", recorder.Code)
	}
}
