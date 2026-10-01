package admind

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

func writeNamedTestFile(t *testing.T, directory string, name string, contents string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if errorValue := os.WriteFile(path, []byte(contents), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func blueclawServingRuns(t *testing.T, runsByCall ...[]taskNotifyRun) (*httptest.Server, *int) {
	t.Helper()
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/admin/api/run/detail" {
			writer.Write([]byte(`{"taskEvents":[]}`))
			return
		}
		if request.URL.Path != "/admin/api/run" {
			http.NotFound(writer, request)
			return
		}
		runs := runsByCall[len(runsByCall)-1]
		if call < len(runsByCall) {
			runs = runsByCall[call]
		}
		call++
		if errorValue := json.NewEncoder(writer).Encode(runs); errorValue != nil {
			t.Error(errorValue)
		}
	}))
	t.Cleanup(server.Close)
	return server, &call
}

func companyDirectoryServing(t *testing.T, members ...centralplane.Member) *httptest.Server {
	t.Helper()
	directory := companyDirectoryHolding(members...)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/agent/member" {
			http.Error(writer, "the record is down", http.StatusBadGateway)
			return
		}
		response, _ := directory.respond(t, request)
		defer response.Body.Close()
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(response.StatusCode)
		_, _ = io.Copy(writer, response.Body)
	}))
	t.Cleanup(server.Close)
	return server
}

func newTaskNotifyCycleService(t *testing.T, blueclawURL string, stateDirectory string) *Service {
	t.Helper()
	return NewService(Configuration{
		DatabasePath:    filepath.Join(stateDirectory, "internkim.sqlite"),
		BlueclawBaseURL: blueclawURL,
	})
}

func TestACycleAdoptsOnceAndThenWatchesForChange(t *testing.T) {
	server, _ := blueclawServingRuns(t, []taskNotifyRun{
		{TaskRunID: "run-1", Status: "running", RequesterPersonID: "person-1", Prompt: "정리해줘"},
	})
	stateDirectory := t.TempDir()
	service := newTaskNotifyCycleService(t, server.URL, stateDirectory)
	ctx := context.Background()

	runs, errorValue := service.taskRunsToConsider(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(runs) != 1 || runs[0].TaskRunID != "run-1" || runs[0].Status != "running" {
		t.Fatalf("runs = %+v", runs)
	}

	service.adoptTaskRunsWithoutNotifying(ctx, runs, runs[0].UpdatedAt)
	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["run-1"] != "running" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestARunNobodyAnswersForIsMarkedAndLeftAlone(t *testing.T) {
	now := time.Now()
	server, _ := blueclawServingRuns(t, []taskNotifyRun{
		{TaskRunID: "orphan", Status: "completed", RequesterPersonID: "nobody", UpdatedAt: now},
	})
	service := newTaskNotifyCycleService(t, server.URL, t.TempDir())
	ctx := context.Background()

	runs, errorValue := service.taskRunsToConsider(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.notifyChangedTaskRuns(ctx, nil, runs, map[string]string{}, now)

	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["orphan"] != "completed" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestAnUnchangedStatusIsNotRevisited(t *testing.T) {
	server, _ := blueclawServingRuns(t, []taskNotifyRun{
		{TaskRunID: "waiting", Status: "waiting_approval", RequesterPersonID: "person-1"},
	})
	service := newTaskNotifyCycleService(t, server.URL, t.TempDir())
	ctx := context.Background()

	runs, errorValue := service.taskRunsToConsider(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.notifyChangedTaskRuns(ctx, nil, runs, map[string]string{"waiting": "waiting_approval"}, runs[0].UpdatedAt)
}

func TestTheCycleDoesNothingWithoutACentralPlane(t *testing.T) {
	server, callCount := blueclawServingRuns(t, []taskNotifyRun{
		{TaskRunID: "run-1", Status: "completed", RequesterPersonID: "person-1"},
	})
	service := newTaskNotifyCycleService(t, server.URL, t.TempDir())

	service.notifyTaskRunsOnce(context.Background())

	if *callCount != 0 {
		t.Fatalf("a device with nowhere to send should not even ask blueclaw, asked %d times", *callCount)
	}
}

func TestARunThatChangedLongAgoIsMarkedWithoutNotifying(t *testing.T) {
	notified := 0
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		notified++
		writer.Write([]byte(`{"told":1,"reached":1}`))
	}))
	t.Cleanup(plane.Close)
	client := centralplane.New(centralplane.Settings{ProjectURL: plane.URL, HostCredential: func() string { return "agent-key" }})

	now := time.Now()
	server, _ := blueclawServingRuns(t, []taskNotifyRun{
		{TaskRunID: "stale", Status: "completed", RequesterPersonID: "person-1", UpdatedAt: now.Add(-taskNotifyFreshFor - time.Minute)},
		{TaskRunID: "fresh", Status: "completed", RequesterPersonID: "person-1", UpdatedAt: now.Add(-time.Minute)},
	})
	directory := companyDirectoryServing(t, centralplane.Member{MemberID: "person-1", Email: "member1@example.com", Status: "active"})
	state := t.TempDir()
	service := NewService(Configuration{
		DatabasePath:               filepath.Join(state, "internkim.sqlite"),
		BlueclawBaseURL:            server.URL,
		CentralPlaneAppURL:         directory.URL,
		CentralPlaneProjectURL:     companyProjectURLForTest,
		CentralPlanePublishableKey: "publishable",
		CentralPlaneAgentKeyPath:   writeNamedTestFile(t, state, "agent-key", "agent-key"),
	})
	ctx := context.Background()

	runs, errorValue := service.taskRunsToConsider(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.notifyChangedTaskRuns(ctx, client, runs, map[string]string{}, now)

	if notified != 1 {
		t.Fatalf("notified %d times, want only the fresh run", notified)
	}
	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["stale"] != "completed" || marks["fresh"] != "completed" {
		t.Fatalf("marks = %+v", marks)
	}
}
