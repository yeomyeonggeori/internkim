package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func blueclawServingRuns(t *testing.T, runsByCall ...[]taskNotifyRun) (*httptest.Server, *int) {
	t.Helper()
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/admin/api/task/detail" {
			writer.Write([]byte(`{"taskEvents":[]}`))
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

func newTaskNotifyCycleService(t *testing.T, blueclawURL string, stateDirectory string) *Service {
	t.Helper()
	return NewService(Configuration{
		DatabasePath:    filepath.Join(stateDirectory, "internkim.sqlite"),
		BlueclawBaseURL: blueclawURL,
	})
}

// The plane is unreachable in a test, so this proves the shape of the cycle:
// what it adopts, what it revisits, and what it leaves alone.
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

// An unresolvable requester still moves the mark, or the same run is retried
// every minute forever.
func TestARunNobodyAnswersForIsMarkedAndLeftAlone(t *testing.T) {
	server, _ := blueclawServingRuns(t, []taskNotifyRun{
		{TaskRunID: "orphan", Status: "completed", RequesterPersonID: "nobody"},
	})
	service := newTaskNotifyCycleService(t, server.URL, t.TempDir())
	ctx := context.Background()

	runs, errorValue := service.taskRunsToConsider(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service.notifyChangedTaskRuns(ctx, nil, runs, map[string]string{}, runs[0].UpdatedAt)

	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["orphan"] != "completed" {
		t.Fatalf("marks = %+v", marks)
	}
}

// A status the poller has already acted on is not looked at again, which is
// what keeps an approval that sits for hours from buzzing every minute.
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
	// nil client would panic if the run were treated as changed.
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
