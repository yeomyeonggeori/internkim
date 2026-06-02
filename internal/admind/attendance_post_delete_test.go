package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestMattermostPostDeleteRemovesAttendanceEvent(t *testing.T) {
	mattermostServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete || request.URL.Path != "/api/v4/posts/result-post-1" {
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer mattermostServer.Close()
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		AttendanceDatabasePath: filepath.Join(stateDirectory, "attendance.sqlite"),
		MattermostBaseURL:      mattermostServer.URL,
		AdminEmailPath:         writeTestFile(t, "admin@example.com"),
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
		MattermostTokenPath:    writeTestFile(t, "bot-token"),
	})
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := service.createAttendanceEvent(
		mattermostUserRecord{ID: "user-1", Username: "staff", Email: "staff@example.com"},
		attendanceKindClockIn,
		time.Now().UTC(),
		"team-1",
		"attendance-channel",
		"action-post-1",
		"result-post-1",
		attendanceLocation{},
	)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v4/posts/result-post-1", nil)
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("delete status = %d", response.Code)
	}
	events, errorValue := service.readAttendanceEvents(context.Background(), time.Now().Format("2006-01"), "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != 0 {
		t.Fatalf("events = %+v", events)
	}
}

func TestMattermostPostDeleteProtectsAttendanceEntryPost(t *testing.T) {
	mattermostServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.Path)
	}))
	defer mattermostServer.Close()
	stateDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         stateDirectory,
		MattermostBaseURL:      mattermostServer.URL,
		MattermostBotTokenPath: writeTestFile(t, "bot-token"),
	})
	service.saveMattermostAttendanceEntryPostID("entry-post")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodDelete, "/api/v4/posts/entry-post", nil)
	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("delete status = %d", response.Code)
	}
}
