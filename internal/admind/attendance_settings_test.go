package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttendanceTeamViewVisibilityRoundtrip(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	ctx := context.Background()

	if errorValue := service.writeAttendanceTeamViewVisibleToAll(ctx, false); errorValue != nil {
		t.Fatal(errorValue)
	}

	visible, errorValue := service.readAttendanceTeamViewVisibleToAll(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if visible {
		t.Fatalf("expected visible=false, got true")
	}
}

func TestAttendanceSettingsToggle(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPatch, "/attendance/api/settings", strings.NewReader(`{"teamViewVisibleToAll":false}`))
	request.RemoteAddr = "127.0.0.1:1234"
	service.handleAttendance(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatal(errorValue)
	}
	if body["teamViewVisibleToAll"] != false {
		t.Fatalf("response = %+v", body)
	}
}
