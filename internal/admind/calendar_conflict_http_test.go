package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestCalendarConflictHTTPListAndDismiss(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	if errorValue := service.recordCalendarConflict(ctx, "event-1", "event-1@internkim", calendarFieldTitle, "Local", "Remote"); errorValue != nil {
		t.Fatalf("record conflict: %v", errorValue)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/conflicts", nil)
	listRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	listResponse := httptest.NewRecorder()
	service.router().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", listResponse.Code, listResponse.Body.String())
	}
	var listDocument calendarConflictsResponse
	if errorValue := json.Unmarshal(listResponse.Body.Bytes(), &listDocument); errorValue != nil {
		t.Fatalf("decode list response: %v", errorValue)
	}
	if len(listDocument.Conflicts) != 1 {
		t.Fatalf("conflict response count: got %d, want 1", len(listDocument.Conflicts))
	}
	if listDocument.Conflicts[0].Field != calendarFieldTitle {
		t.Fatalf("conflict field: got %q", listDocument.Conflicts[0].Field)
	}

	conflictID := strconv.FormatInt(listDocument.Conflicts[0].ID, 10)
	dismissRequest := httptest.NewRequest(http.MethodPost, "/calendar/api/conflicts/"+conflictID+"/dismiss", nil)
	dismissRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	dismissResponse := httptest.NewRecorder()
	service.router().ServeHTTP(dismissResponse, dismissRequest)
	if dismissResponse.Code != http.StatusNoContent {
		t.Fatalf("dismiss status = %d body = %s", dismissResponse.Code, dismissResponse.Body.String())
	}

	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatalf("list active conflicts: %v", errorValue)
	}
	if len(conflicts) != 0 {
		t.Fatalf("active conflicts after dismiss: got %d, want 0", len(conflicts))
	}
}
