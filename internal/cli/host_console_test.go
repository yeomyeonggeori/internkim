package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type hostConsoleRecordingExecutor struct {
	mutex         sync.Mutex
	path          string
	runErrors     []error
	runs          []hostCommandInvocation
	outputs       []hostCommandInvocation
	outputText    map[string]string
	runOutputText map[string]string
}

func (executor *hostConsoleRecordingExecutor) LookPath(name string) (string, error) {
	if name != "container" {
		return "", errors.New("unexpected executable: " + name)
	}
	if executor.path == "" {
		return "", errors.New("missing executable")
	}
	return executor.path, nil
}

func (executor *hostConsoleRecordingExecutor) CombinedOutput(invocation hostCommandInvocation) ([]byte, error) {
	executor.mutex.Lock()
	defer executor.mutex.Unlock()
	executor.outputs = append(executor.outputs, copyHostCommandInvocation(invocation))
	if value, ok := executor.outputText[hostCommandInvocationKey(invocation)]; ok {
		return []byte(value), nil
	}
	return []byte{}, nil
}

func (executor *hostConsoleRecordingExecutor) Run(invocation hostCommandInvocation) error {
	executor.mutex.Lock()
	executor.runs = append(executor.runs, copyHostCommandInvocation(invocation))
	outputText := executor.runOutputText[hostCommandInvocationKey(invocation)]
	errorValue := error(nil)
	if len(executor.runErrors) > 0 {
		errorValue = executor.runErrors[0]
		executor.runErrors = executor.runErrors[1:]
	}
	executor.mutex.Unlock()
	if outputText != "" && invocation.Stdout != nil {
		_, _ = invocation.Stdout.Write([]byte(outputText))
	}
	return errorValue
}

func TestHostConsoleTenantsEndpointShape(t *testing.T) {
	executor := newHostConsoleTenantsExecutor()
	handler := newHostConsoleServer(executor, "vm-1").handler()

	response := getHostConsoleResponse(handler, "/api/tenants")

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", response.StatusCode)
	}
	var tenants []hostTenantSummary
	if errorValue := json.NewDecoder(response.Body).Decode(&tenants); errorValue != nil {
		t.Fatal(errorValue)
	}
	expected := []hostTenantSummary{{TenantID: "pilot-01", PublicURL: "https://pilot.example.com", Running: true}}
	if !reflect.DeepEqual(tenants, expected) {
		t.Fatalf("unexpected tenants:\nwant: %+v\n got: %+v", expected, tenants)
	}
}

func TestHostConsoleSuccessfulJobLifecycle(t *testing.T) {
	executor := newHostConsoleTenantsExecutor()
	provisionOutput := `{"tenantID":"pilot-01","publicURL":"https://pilot.example.com","members":[{"email":"owner@example.com","name":"Owner","password":"generated-password","passwordGenerated":true}]}`
	executor.runOutputText = map[string]string{
		hostCommandInvocationKey(hostAddTeamInvocation("container", hostAddTeamOptions{
			TeamID:             "pilot-01",
			VirtualMachineName: "vm-1",
			RemoteArguments: []string{
				"--display-name", "Pilot",
				"--member", "owner@example.com:Owner:",
			},
		})): provisionOutput + "\n",
	}
	handler := newHostConsoleServer(executor, "vm-1").handler()

	jobID := postHostConsoleTeam(t, handler, `{
		"teamID": "pilot-01",
		"displayName": "Pilot",
		"members": [{"email": "owner@example.com", "name": "Owner", "password": ""}]
	}`, http.StatusAccepted)
	job := waitForHostConsoleJob(t, handler, jobID, "completed")

	if !strings.Contains(job.Log, "generated-password") {
		t.Fatalf("expected provision output in log, got %q", job.Log)
	}
	var summary tenantProvisionSummary
	if errorValue := json.Unmarshal(job.Summary, &summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if summary.TenantID != "pilot-01" || summary.PublicURL != "https://pilot.example.com" {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Members) != 1 || !summary.Members[0].IsPasswordGenerated || summary.Members[0].Password != "generated-password" {
		t.Fatalf("unexpected member summary: %+v", summary.Members)
	}
}

func TestHostConsoleFailedJobLifecycle(t *testing.T) {
	executor := newHostConsoleTenantsExecutor()
	executor.runErrors = []error{nil, nil, errors.New("remote provision failed")}
	handler := newHostConsoleServer(executor, "vm-1").handler()

	jobID := postHostConsoleTeam(t, handler, `{
		"teamID": "pilot-01",
		"displayName": "Pilot",
		"members": [{"email": "owner@example.com", "name": "Owner", "password": "secret"}]
	}`, http.StatusAccepted)
	job := waitForHostConsoleJob(t, handler, jobID, "failed")

	if !strings.Contains(job.Log, "remote provision failed") {
		t.Fatalf("expected failure in log, got %q", job.Log)
	}
}

func TestHostConsoleRejectsConcurrentTeamJob(t *testing.T) {
	console := newHostConsoleServer(newHostConsoleTenantsExecutor(), "vm-1")
	console.jobs.jobs["existing"] = hostConsoleJob{JobID: "existing", Status: "running"}
	handler := console.handler()

	postHostConsoleTeam(t, handler, `{
		"teamID": "pilot-01",
		"displayName": "Pilot",
		"members": [{"email": "owner@example.com", "name": "Owner"}]
	}`, http.StatusConflict)
}

func TestHostConsoleRejectsInvalidInput(t *testing.T) {
	handler := newHostConsoleServer(newHostConsoleTenantsExecutor(), "vm-1").handler()

	tests := []struct {
		name string
		body string
	}{
		{
			name: "team id",
			body: `{"teamID":"Pilot 01","displayName":"Pilot","members":[{"email":"owner@example.com","name":"Owner"}]}`,
		},
		{
			name: "email",
			body: `{"teamID":"pilot-01","displayName":"Pilot","members":[{"email":"bad","name":"Owner"}]}`,
		},
		{
			name: "name",
			body: `{"teamID":"pilot-01","displayName":"Pilot","members":[{"email":"owner@example.com","name":""}]}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			postHostConsoleTeam(t, handler, test.body, http.StatusBadRequest)
		})
	}
}

func postHostConsoleTeam(t *testing.T, handler http.Handler, body string, expectedStatusCode int) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/teams", bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != expectedStatusCode {
		responseBody, _ := io.ReadAll(response.Result().Body)
		t.Fatalf("expected status %d, got %d: %s", expectedStatusCode, response.Code, string(responseBody))
	}
	if expectedStatusCode != http.StatusAccepted {
		return ""
	}
	var payload struct {
		JobID string `json:"jobID"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.JobID == "" {
		t.Fatal("expected job id")
	}
	return payload.JobID
}

func waitForHostConsoleJob(t *testing.T, handler http.Handler, jobID string, expectedStatus string) hostConsoleJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		response := getHostConsoleResponse(handler, "/api/jobs/"+jobID)
		var job hostConsoleJob
		errorValue := json.NewDecoder(response.Body).Decode(&job)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if job.Status == expectedStatus {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for job %s", expectedStatus)
	return hostConsoleJob{}
}

func getHostConsoleResponse(handler http.Handler, path string) *http.Response {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Result()
}

func newHostConsoleTenantsExecutor() *hostConsoleRecordingExecutor {
	executor := &hostConsoleRecordingExecutor{
		path:          "container",
		outputText:    map[string]string{},
		runOutputText: map[string]string{},
	}
	executor.outputText[hostCommandInvocationKey(hostContainerExecInvocation("container", "vm-1", "ls", "-1", "/srv/internkim/tenants"))] = "pilot-01\n"
	executor.outputText[hostCommandInvocationKey(hostContainerExecInvocation("container", "vm-1", "internkim", "tenant", "status", "--tenant", "pilot-01"))] = `{"manifest":{"tenantID":"pilot-01","publicURL":"https://pilot.example.com"}}`
	executor.outputText[hostCommandInvocationKey(hostContainerExecInvocation("container", "vm-1", "systemctl", "is-active", "internkim-tenant-blueclaw-pilot-01.service"))] = "active\n"
	return executor
}
