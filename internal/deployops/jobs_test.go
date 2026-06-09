package deployops

import (
	"sync"
	"testing"
	"time"
)

func TestJobStoreLocksTarget(t *testing.T) {
	store := NewJobStore()
	releaseJob := make(chan struct{})
	startedJob := make(chan struct{})

	firstJob, errorValue := store.Start("pilot-01", JobActionDeployAdmind, func(job *JobRunner) {
		close(startedJob)
		<-releaseJob
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	<-startedJob

	if _, errorValue := store.Start("pilot-01", JobActionDeployWeb, func(job *JobRunner) {}); errorValue == nil {
		t.Fatal("expected second job on same target to fail")
	}
	close(releaseJob)
	waitForJobState(t, store, firstJob.ID, "succeeded")
}

func TestJobStoreAllowsDifferentTargets(t *testing.T) {
	store := NewJobStore()
	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	_, firstError := store.Start("pilot-01", JobActionCheck, func(job *JobRunner) { waitGroup.Done() })
	_, secondError := store.Start("pilot-02", JobActionCheck, func(job *JobRunner) { waitGroup.Done() })
	if firstError != nil || secondError != nil {
		t.Fatalf("expected different targets to run: %v %v", firstError, secondError)
	}
	waitGroup.Wait()
}

func waitForJobState(t *testing.T, store *JobStore, jobID string, state string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := store.Get(jobID)
		if ok && job.State == state {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %s did not reach %s", jobID, state)
}
