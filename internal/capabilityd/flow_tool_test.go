package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestResolveFlowOwnerFindsNameInsidePrompt(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "lee", Email: "lee@example.com"},
		{ID: "iam", Name: "iam", Email: "iam@example.com"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{Prompt: "lee에게 10분 회의 추가해줘"}, "", members)
	if resolution.OwnerID != "lee" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveFlowOwnerMatchesMattermostHandle(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "Lee Dongha", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "Kim Dongha", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{Prompt: "@kim에게 업무 요청해줘"}, "", members)
	if resolution.OwnerID != "kim" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveFlowOwnerMatchesTargetPersonHint(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee-1", Name: "이샘플", Email: "lee@example.com", MattermostUsername: "lee"},
	}
	for _, testCase := range []struct {
		name  string
		value string
	}{
		{name: "handle", value: "@lee"},
		{name: "name", value: "이샘플"},
		{name: "email", value: "lee@example.com"},
		{name: "member id", value: "lee-1"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			resolution := resolveFlowOwner(flowTaskAddInput{Prompt: "업무 요청해줘", TargetPersonHint: testCase.value}, "", members)
			if resolution.OwnerID != "lee-1" {
				t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
			}
		})
	}
}

func TestResolveFlowOwnerFallsBackToRequesterEmail(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "staff", Name: "Staff", Email: "staff@example.com", MattermostUsername: "staff"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{Prompt: "10분 회의 추가해줘"}, "staff@example.com", members)
	if resolution.OwnerID != "staff" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveFlowOwnerReturnsAmbiguousCandidates(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "샘플", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "샘플", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{Prompt: "샘플에게 업무 요청해줘"}, "", members)
	if resolution.Failure == nil || resolution.Failure.ErrorCode != "flow_owner_ambiguous" {
		t.Fatalf("failure = %+v", resolution.Failure)
	}
	if len(resolution.Failure.Candidates) != 2 || resolution.Failure.Candidates[0].Mention != "@lee" || resolution.Failure.Candidates[1].Mention != "@kim" {
		t.Fatalf("candidates = %+v", resolution.Failure.Candidates)
	}
}

func TestFlowTaskAddPropagatesRequesterEmail(t *testing.T) {
	var summaryRequesterEmail string
	var taskRequesterEmail string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary":
				summaryRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				taskRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
				return flowToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.add",
		Input:    []byte(`{"prompt":"10분 회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if summaryRequesterEmail != "staff@example.com" || taskRequesterEmail != "staff@example.com" {
		t.Fatalf("requester headers summary=%q task=%q", summaryRequesterEmail, taskRequesterEmail)
	}
}

func TestFlowTaskAddPropagatesDuplicateConfirmation(t *testing.T) {
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return flowToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.add",
		Input:    []byte(`{"prompt":"10분 회의","allowDuplicate":true}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["allowDuplicate"] != true {
		t.Fatalf("allowDuplicate = %#v", payload["allowDuplicate"])
	}
}

func TestFlowTaskAddReturnsAmbiguousOwnerError(t *testing.T) {
	postCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary":
				return flowToolJSONResponse(`{"members":[{"id":"lee","name":"샘플","email":"lee@example.com","mattermostUsername":"lee"},{"id":"kim","name":"샘플","email":"kim@example.com","mattermostUsername":"kim"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				postCalled = true
				return flowToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.add",
		Input:    []byte(`{"prompt":"샘플에게 업무 요청해줘"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_owner_ambiguous" {
		t.Fatalf("response = %+v", response)
	}
	if postCalled {
		t.Fatal("post should not be called for ambiguous owner")
	}
	if !strings.Contains(string(response.Result), "@lee") || !strings.Contains(string(response.Result), "@kim") {
		t.Fatalf("result = %s", string(response.Result))
	}
}

func TestFlowTaskAddReturnsSkippedDuplicateStatus(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				return flowToolJSONResponse(`{"status":"skipped_duplicate","duplicateTask":{"id":"task-1"}}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.add",
		Input:    []byte(`{"prompt":"10분 회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "skipped_duplicate" {
		t.Fatalf("status = %q", response.Status)
	}
}

func flowToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
