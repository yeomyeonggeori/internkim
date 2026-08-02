package companion

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type jobRunnerTransport func(request *http.Request) (*http.Response, error)

func (transport jobRunnerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

type jobRunnerExecutor struct {
	response       capabilities.ToolInvokeResponse
	executionError error
	envelope       JobEnvelope
	request        capabilities.ToolInvokeRequest
}

func (executor *jobRunnerExecutor) ExecuteJob(ctx context.Context, envelope JobEnvelope, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	_ = ctx
	executor.envelope = envelope
	executor.request = request
	return executor.response, executor.executionError
}

type jobRunnerRuntime struct {
	heartbeatErrors []error
}

func (runtime *jobRunnerRuntime) LocalLLMAvailable() bool {
	return true
}

func (runtime *jobRunnerRuntime) RecordHeartbeat(errorValue error) {
	runtime.heartbeatErrors = append(runtime.heartbeatErrors, errorValue)
}

func TestJobRunnerCompletesClaimedJob(t *testing.T) {
	keyPair, errorValue := GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	paths := []string{}
	var heartbeatPayload map[string]any
	transport := jobRunnerTransport(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get(SignatureHeader) == "" {
			t.Fatal("expected signed companion request")
		}
		paths = append(paths, request.URL.Path)
		switch request.URL.Path {
		case "/_internkim/companion/heartbeat":
			if errorValue := json.NewDecoder(request.Body).Decode(&heartbeatPayload); errorValue != nil {
				t.Fatal(errorValue)
			}
			return textJobRunnerResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textJobRunnerResponse(http.StatusOK, `{"jobID":"job-1","status":"running","toolName":"user.input","request":{"toolName":"user.input","input":{"message":"Name"}}}`), nil
		case "/_internkim/companion/jobs/job-1/complete":
			return textJobRunnerResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})
	executor := &jobRunnerExecutor{response: capabilities.ToolInvokeResponse{ToolName: "user.input", Content: "Lee"}}
	runtime := &jobRunnerRuntime{}
	runner := JobRunner{
		DeviceClient: DeviceClient{
			HTTPClient: &http.Client{Transport: transport},
			State:      State{DeviceURL: "https://device.example.test", CompanionID: "companion-1", Token: "token-1", LocalOnly: true},
			PrivateKey: keyPair.PrivateKey,
		},
		Executor: executor,
		Runtime:  runtime,
		RunOnce:  true,
	}

	if errorValue := runner.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(paths, ",") != "/_internkim/companion/heartbeat,/_internkim/companion/jobs/next,/_internkim/companion/jobs/job-1/complete" {
		t.Fatalf("unexpected paths: %v", paths)
	}
	if heartbeatPayload["localLLMAvailable"] != true {
		t.Fatalf("expected local LLM availability in heartbeat, got %+v", heartbeatPayload)
	}
	if len(runtime.heartbeatErrors) != 1 || runtime.heartbeatErrors[0] != nil {
		t.Fatalf("expected successful heartbeat record, got %+v", runtime.heartbeatErrors)
	}
	if executor.envelope.JobID != "job-1" || executor.envelope.ToolName != "user.input" {
		t.Fatalf("unexpected executor envelope: %+v", executor.envelope)
	}
}

func TestJobRunnerRoutesDenial(t *testing.T) {
	keyPair, errorValue := GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	seenDeny := false
	transport := jobRunnerTransport(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/_internkim/companion/heartbeat":
			return textJobRunnerResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textJobRunnerResponse(http.StatusOK, `{"jobID":"job-1","status":"running","toolName":"browser_click","request":{"toolName":"browser_click","input":{}}}`), nil
		case "/_internkim/companion/jobs/job-1/deny":
			seenDeny = true
			return textJobRunnerResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})
	denial := capabilities.DenialResult{Status: "denied", Code: "not_allowed", JobID: "job-1", ToolName: "browser_click"}
	runner := JobRunner{
		DeviceClient: DeviceClient{
			HTTPClient: &http.Client{Transport: transport},
			State:      State{DeviceURL: "https://device.example.test", CompanionID: "companion-1", Token: "token-1"},
			PrivateKey: keyPair.PrivateKey,
		},
		Executor: &jobRunnerExecutor{executionError: DenialError{Denial: denial}},
		RunOnce:  true,
	}

	errorValue = runner.Run(context.Background())

	var denialError DenialError
	if !errors.As(errorValue, &denialError) {
		t.Fatalf("expected denial error, got %v", errorValue)
	}
	if !seenDeny {
		t.Fatal("expected denial route")
	}
}

func TestJobRunnerStopsWhenRunOnceFindsNoJob(t *testing.T) {
	keyPair, errorValue := GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requestCount := 0
	transport := jobRunnerTransport(func(request *http.Request) (*http.Response, error) {
		requestCount++
		switch request.URL.Path {
		case "/_internkim/companion/heartbeat":
			return textJobRunnerResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textJobRunnerResponse(http.StatusOK, `{"status":"empty"}`), nil
		default:
			t.Fatalf("unexpected path: %s", request.URL.Path)
			return nil, nil
		}
	})
	runner := JobRunner{
		DeviceClient: DeviceClient{
			HTTPClient: &http.Client{Transport: transport},
			State:      State{DeviceURL: "https://device.example.test", CompanionID: "companion-1", Token: "token-1"},
			PrivateKey: keyPair.PrivateKey,
		},
		Executor: &jobRunnerExecutor{},
		RunOnce:  true,
	}

	if errorValue := runner.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestCount != 2 {
		t.Fatalf("expected heartbeat and poll only, got %d requests", requestCount)
	}
}

func textJobRunnerResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}
