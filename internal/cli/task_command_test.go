package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

type fakeTaskAdminAPIClient struct {
	taskRuns []commandTaskRun
	detail   commandTaskDetail
	requests []string
}

func (client *fakeTaskAdminAPIClient) request(method string, path string, requestBody any, responseBody any) ([]byte, error) {
	client.requests = append(client.requests, method+" "+path)
	payload := any(client.taskRuns)
	if strings.HasPrefix(path, "/diagnostics/task-detail") {
		payload = client.detail
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	if responseBody != nil {
		if errorValue := json.Unmarshal(document, responseBody); errorValue != nil {
			return nil, errorValue
		}
	}
	return document, nil
}

func withCapturedTaskCommandOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	previousOutput := taskCommandOutput
	buffer := &bytes.Buffer{}
	taskCommandOutput = buffer
	t.Cleanup(func() {
		taskCommandOutput = previousOutput
	})
	return buffer
}

func TestTaskListFiltersFailedRuns(t *testing.T) {
	output := withCapturedTaskCommandOutput(t)
	client := &fakeTaskAdminAPIClient{taskRuns: []commandTaskRun{
		{TaskRunID: "task-completed", Status: "completed", Prompt: "summarize", UpdatedAt: time.Now()},
		{TaskRunID: "task-failed", Status: "failed", FailureReason: "tool denied", UpdatedAt: time.Now().Add(-time.Minute)},
	}}

	errorValue := runTaskArgumentsWithClient([]string{"list", "--failed"}, client)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if client.requests[0] != "GET /diagnostics/tasks?limit=20&status=failed" {
		t.Fatalf("requests = %v", client.requests)
	}
	if !strings.Contains(output.String(), "task-failed") || strings.Contains(output.String(), "task-completed") {
		t.Fatalf("output = %s", output.String())
	}
}

func TestTaskLogsLastFailedFetchesMostRecentFailedDetail(t *testing.T) {
	output := withCapturedTaskCommandOutput(t)
	client := &fakeTaskAdminAPIClient{
		taskRuns: []commandTaskRun{
			{TaskRunID: "task-new-completed", Status: "completed", UpdatedAt: time.Now()},
			{TaskRunID: "task-old-failed", Status: "failed", UpdatedAt: time.Now().Add(-2 * time.Hour)},
			{TaskRunID: "task-recent-failed", Status: "failed", UpdatedAt: time.Now().Add(-time.Hour)},
		},
		detail: commandTaskDetail{
			TaskRun: commandTaskRun{TaskRunID: "task-recent-failed", Status: "failed", FailureReason: "terminal denied"},
			TaskEvents: []commandTaskEvent{
				{Name: "agent.failure_reply", Body: `{"code":"denied"}`, CreatedAt: time.Now()},
			},
		},
	}

	errorValue := runTaskArgumentsWithClient([]string{"logs", "--last-failed"}, client)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if client.requests[1] != "GET /diagnostics/task-detail?taskRunID=task-recent-failed" {
		t.Fatalf("requests = %v", client.requests)
	}
	if !strings.Contains(output.String(), "terminal denied") || !strings.Contains(output.String(), `{"code":"denied"}`) {
		t.Fatalf("output = %s", output.String())
	}
}

func TestTaskLogsResolvesUniquePrefix(t *testing.T) {
	withCapturedTaskCommandOutput(t)
	client := &fakeTaskAdminAPIClient{
		taskRuns: []commandTaskRun{
			{TaskRunID: "task-aaa", Status: "completed"},
			{TaskRunID: "task-bbb", Status: "failed"},
		},
		detail: commandTaskDetail{TaskRun: commandTaskRun{TaskRunID: "task-bbb"}},
	}

	errorValue := runTaskArgumentsWithClient([]string{"logs", "task-b"}, client)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if client.requests[1] != "GET /diagnostics/task-detail?taskRunID=task-bbb" {
		t.Fatalf("requests = %v", client.requests)
	}
}

func TestTaskLogsRejectsAmbiguousPrefix(t *testing.T) {
	withCapturedTaskCommandOutput(t)
	client := &fakeTaskAdminAPIClient{taskRuns: []commandTaskRun{
		{TaskRunID: "task-aa1"},
		{TaskRunID: "task-aa2"},
	}}

	errorValue := runTaskArgumentsWithClient([]string{"logs", "task-aa"}, client)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "ambiguous") {
		t.Fatalf("errorValue = %v", errorValue)
	}
}

func TestTaskLogsRequiresTargetSelection(t *testing.T) {
	withCapturedTaskCommandOutput(t)
	client := &fakeTaskAdminAPIClient{}

	errorValue := runTaskArgumentsWithClient([]string{"logs"}, client)

	if errorValue == nil || !strings.Contains(errorValue.Error(), "usage") {
		t.Fatalf("errorValue = %v", errorValue)
	}
	if len(client.requests) != 0 {
		t.Fatalf("requests = %v", client.requests)
	}
}
