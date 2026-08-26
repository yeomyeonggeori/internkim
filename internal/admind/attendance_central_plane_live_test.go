package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestAClockInReachesThePlaneAsTheMemberWhoClicked(t *testing.T) {
	var carried map[string]any
	var carriedToken string
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/api/agent/session") {
			var asked map[string]string
			json.NewDecoder(request.Body).Decode(&asked)
			if asked["kind"] != "mattermost" || asked["externalID"] != "U1" {
				t.Errorf("the plane must be told who spoke, got %v", asked)
			}
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"member-token","expiresAt":` +
				strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`))
			return
		}
		carriedToken = request.Header.Get("Authorization")
		json.NewDecoder(request.Body).Decode(&carried)
		writer.WriteHeader(http.StatusCreated)
	}))
	defer plane.Close()

	service, _ := newAttendanceActionTestService(t)
	forgetCentralPlaneForTest(service)
	service.Configuration.CentralPlaneAppURL = plane.URL
	service.Configuration.CentralPlaneProjectURL = plane.URL
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")

	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	event := attendanceEvent{
		ID:               "event-live",
		MattermostUserID: "U1",
		Email:            "someone@example.test",
		Kind:             attendanceKindClockIn,
		OccurredAt:       time.Now().UTC().Format(time.RFC3339),
		LocalDate:        "2026-08-04",
		LocalTime:        "09:00",
		TimeZoneAtEvent:  "Asia/Seoul",
		Source:           "mattermost",
		LocationName:     "사무실",
	}
	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
		t.Fatalf("the clock-in failed: %v", errorValue)
	}

	deadline := time.Now().Add(5 * time.Second)
	for carried == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if carried == nil {
		t.Fatal("the plane never heard about the clock-in")
	}
	if carriedToken != "Bearer member-token" {
		t.Fatalf("the plane must be written as the member, got %q", carriedToken)
	}
	if carried["member_id"] != "member-1" || carried["kind"] != "clock_in" || carried["location"] != "사무실" {
		t.Fatalf("the record lost something on the way: %v", carried)
	}
}

func writeAgentKeyForTest(t testing.TB, key string) string {
	t.Helper()
	path := t.TempDir() + "/agent-key"
	if errorValue := os.WriteFile(path, []byte(key), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}
