package deployops

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type JobStore struct {
	mutex       sync.Mutex
	jobs        map[string]*Job
	watchers    map[string]map[chan JobEvent]bool
	targetLocks map[string]bool
	nextID      int
	nextEventID int
}

type JobRunner struct {
	store *JobStore
	jobID string
}

func NewJobStore() *JobStore {
	return &JobStore{
		jobs:        map[string]*Job{},
		watchers:    map[string]map[chan JobEvent]bool{},
		targetLocks: map[string]bool{},
	}
}

func (store *JobStore) Start(targetID string, action string, run func(job *JobRunner)) (Job, error) {
	store.mutex.Lock()
	if store.targetLocks[targetID] {
		store.mutex.Unlock()
		return Job{}, fmt.Errorf("target %s already has a running job", targetID)
	}
	store.nextID++
	jobID := fmt.Sprintf("job-%d", store.nextID)
	job := &Job{
		ID:        jobID,
		TargetID:  targetID,
		Action:    action,
		State:     "running",
		StartedAt: time.Now(),
	}
	store.jobs[jobID] = job
	store.targetLocks[targetID] = true
	store.mutex.Unlock()
	go func() {
		runner := &JobRunner{store: store, jobID: jobID}
		runner.Info("started " + action)
		run(runner)
		store.finish(jobID, "", nil)
	}()
	return cloneJob(job), nil
}

func (store *JobStore) Get(jobID string) (Job, bool) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	job, ok := store.jobs[jobID]
	if !ok {
		return Job{}, false
	}
	return cloneJob(job), true
}

func (store *JobStore) Stream(responseWriter http.ResponseWriter, request *http.Request, jobID string) {
	flusher, ok := responseWriter.(http.Flusher)
	if !ok {
		writeError(responseWriter, http.StatusInternalServerError, "streaming is not available")
		return
	}
	responseWriter.Header().Set("Content-Type", "text/event-stream")
	responseWriter.Header().Set("Cache-Control", "no-cache")
	responseWriter.Header().Set("Connection", "keep-alive")
	eventChannel := make(chan JobEvent, 64)
	events, ok := store.addWatcher(jobID, eventChannel)
	if !ok {
		writeError(responseWriter, http.StatusNotFound, "job not found")
		return
	}
	defer store.removeWatcher(jobID, eventChannel)
	for _, event := range events {
		writeEvent(responseWriter, flusher, event)
	}
	for {
		select {
		case <-request.Context().Done():
			return
		case event := <-eventChannel:
			writeEvent(responseWriter, flusher, event)
		}
	}
}

func (store *JobStore) addWatcher(jobID string, eventChannel chan JobEvent) ([]JobEvent, bool) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	job, ok := store.jobs[jobID]
	if !ok {
		return nil, false
	}
	if store.watchers[jobID] == nil {
		store.watchers[jobID] = map[chan JobEvent]bool{}
	}
	store.watchers[jobID][eventChannel] = true
	return append([]JobEvent(nil), job.Events...), true
}

func (store *JobStore) removeWatcher(jobID string, eventChannel chan JobEvent) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	delete(store.watchers[jobID], eventChannel)
	close(eventChannel)
}

func (store *JobStore) finish(jobID string, errorMessage string, errorValue error) {
	store.mutex.Lock()
	job := store.jobs[jobID]
	if job == nil || job.State != "running" {
		store.mutex.Unlock()
		return
	}
	finishedAt := time.Now()
	job.FinishedAt = &finishedAt
	job.State = "succeeded"
	if errorValue != nil {
		job.State = "failed"
		if errorMessage != "" {
			job.Error = errorMessage
		} else {
			job.Error = errorValue.Error()
		}
	}
	delete(store.targetLocks, job.TargetID)
	store.mutex.Unlock()
	if errorValue != nil {
		store.appendEvent(jobID, "error", job.Error)
		return
	}
	store.appendEvent(jobID, "info", "completed")
}

func (runner *JobRunner) Info(message string) {
	runner.store.appendEvent(runner.jobID, "info", Redact(message))
}

func (runner *JobRunner) Error(errorValue error) {
	if errorValue == nil {
		return
	}
	runner.store.finish(runner.jobID, "", errorValue)
}

func (runner *JobRunner) appendLine(line string) {
	if line == "" {
		return
	}
	runner.Info(Redact(line))
}

func (store *JobStore) appendEvent(jobID string, level string, message string) {
	store.mutex.Lock()
	job := store.jobs[jobID]
	if job == nil {
		store.mutex.Unlock()
		return
	}
	store.nextEventID++
	event := JobEvent{
		ID:      store.nextEventID,
		JobID:   jobID,
		At:      time.Now(),
		Level:   level,
		Message: message,
	}
	job.Events = append(job.Events, event)
	watchers := make([]chan JobEvent, 0, len(store.watchers[jobID]))
	for eventChannel := range store.watchers[jobID] {
		watchers = append(watchers, eventChannel)
	}
	store.mutex.Unlock()
	for _, eventChannel := range watchers {
		select {
		case eventChannel <- event:
		default:
		}
	}
}

func writeEvent(responseWriter http.ResponseWriter, flusher http.Flusher, event JobEvent) {
	document, _ := json.Marshal(event)
	fmt.Fprintf(responseWriter, "id: %d\n", event.ID)
	fmt.Fprintf(responseWriter, "event: message\n")
	fmt.Fprintf(responseWriter, "data: %s\n\n", string(document))
	flusher.Flush()
}

func cloneJob(job *Job) Job {
	clonedJob := *job
	clonedJob.Events = append([]JobEvent(nil), job.Events...)
	return clonedJob
}
