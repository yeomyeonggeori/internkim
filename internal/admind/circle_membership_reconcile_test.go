package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func reconcileResponseFor(t *testing.T, query string) *httptest.ResponseRecorder {
	t.Helper()
	service := NewService(Configuration{})
	request := httptest.NewRequest(http.MethodGet, "/agent/api/circle-membership-reconcile"+query, nil)
	request.RemoteAddr = "127.0.0.1:54321"
	recorder := httptest.NewRecorder()
	service.handleCircleMembershipReconcile(recorder, request)
	return recorder
}

func TestAskingWhichCirclesWithoutNamingAnybodyIsNotABadGateway(t *testing.T) {
	recorder := reconcileResponseFor(t, "")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a caller that named nobody must be told so, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestTheReconcileSaysWhichAddressItNeeds(t *testing.T) {
	recorder := reconcileResponseFor(t, "?email=%20%20")

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("an address of only spaces names nobody, got %d", recorder.Code)
	}
}
