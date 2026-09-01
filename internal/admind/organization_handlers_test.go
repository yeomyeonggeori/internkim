package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrganizationGroupHandlerRejectsHierarchyCycle(t *testing.T) {
	service := newLocalUsersTestService(t)
	requestBody := strings.NewReader(`{"groups":[{"id":"product","name":"제품","parentID":"growth"},{"id":"growth","name":"성장","parentID":"product"}]}`)
	responseRecorder := httptest.NewRecorder()

	service.localSetOrgGroups(responseRecorder, httptest.NewRequest(http.MethodPut, "/admin/api/org-groups", requestBody))

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d; want %d; body = %s", responseRecorder.Code, http.StatusBadRequest, responseRecorder.Body.String())
	}
}
