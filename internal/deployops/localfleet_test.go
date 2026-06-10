package deployops

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLocalFleetJobEndpointUsesSharedJobStore(t *testing.T) {
	server := &Server{
		options: ServerOptions{
			RepositoryRootPath: t.TempDir(),
			ExecutablePath:     "/bin/echo",
		},
		jobs: NewJobStore(),
	}
	body := bytes.NewBufferString(`{"action":"unsupported"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/local-fleet/jobs", body)
	response := httptest.NewRecorder()

	server.routes().ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var job Job
	if errorValue := json.NewDecoder(response.Body).Decode(&job); errorValue != nil {
		t.Fatal(errorValue)
	}
	waitForJobState(t, server.jobs, job.ID, "failed")
	storedJob, ok := server.jobs.Get(job.ID)
	if !ok || storedJob.TargetID != "local-fleet" {
		t.Fatalf("job = %+v ok=%v", storedJob, ok)
	}
}

func TestLocalFleetJobEndpointLocksLocalFleetTarget(t *testing.T) {
	store := NewJobStore()
	releaseJob := make(chan struct{})
	startedJob := make(chan struct{})
	_, errorValue := store.Start("local-fleet", "local-fleet:up", func(job *JobRunner) {
		close(startedJob)
		<-releaseJob
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	<-startedJob
	if _, errorValue := store.Start("local-fleet", "local-fleet:down", func(job *JobRunner) {}); errorValue == nil {
		t.Fatal("expected local fleet target lock")
	}
	close(releaseJob)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, errorValue := store.Start("local-fleet", "local-fleet:down", func(job *JobRunner) {}); errorValue == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("local fleet lock did not release")
}
