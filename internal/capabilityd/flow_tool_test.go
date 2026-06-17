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
		{ID: "lee", Name: "Lee Gamyeong", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "Kim Gamyeong", Email: "kim@example.com", MattermostUsername: "kim"},
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
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
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
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
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
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
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
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
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

func TestFlowTaskListFiltersTasksByQueryIgnoringSpaces(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method != http.MethodGet || request.URL.String() != "http://admind.local/flow/api/state" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			return flowToolJSONResponse(`{"currentWeek":{"code":"26W23"},"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"task-1","ownerID":"staff","ownerName":"Staff","content":"디플랫코리아 기획안 전달","status":"예정","weekCode":"26W23"},{"id":"task-2","ownerID":"staff","ownerName":"Staff","content":"사무실 미팅","status":"예정","weekCode":"26W23"}]}`), nil
		})},
	}

	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.list",
		Input:    []byte(`{"query":"디플랫 코리아"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "ok" {
		t.Fatalf("status = %q", response.Status)
	}
	if !strings.Contains(string(response.Result), "task-1") || strings.Contains(string(response.Result), "task-2") {
		t.Fatalf("result = %s", string(response.Result))
	}
}

func TestFlowTaskUpdateUsesSharedPutAPIWithoutCreatingTask(t *testing.T) {
	postCalled := false
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"business":"개발","type":"회의","content":"10분 회의","goal":"정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://admind.local/flow/api/tasks/task-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return flowToolJSONResponse(`{"id":"task-1","content":"15분 회의","status":"진행"}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				postCalled = true
				return flowToolJSONResponse(`{"id":"new-task"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.update",
		Input:    []byte(`{"taskID":"task-1","content":"15분 회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || response.Status != "진행" {
		t.Fatalf("response = %+v", response)
	}
	if postCalled {
		t.Fatal("update must not call quick create")
	}
	if updatedPayload["content"] != "15분 회의" || updatedPayload["status"] != "진행" || updatedPayload["goal"] != "정리" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestFlowTaskUpdateQueryOnlyDefaultsToComplete(t *testing.T) {
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary?week=26W24":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"business":"개발","type":"회의","content":"10분 회의","goal":"정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://admind.local/flow/api/tasks/task-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return flowToolJSONResponse(`{"id":"task-1","status":"완료"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.update",
		Input:    []byte(`{"query":"10분 회의","weekCode":"26W24"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || response.Status != "완료" {
		t.Fatalf("response = %+v", response)
	}
	if updatedPayload["status"] != "완료" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestFlowTaskUpdateAmbiguousQueryDoesNotWrite(t *testing.T) {
	putCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"type":"회의","content":"회의 준비","size":"XS","status":"진행","weekCode":"26W24"},{"id":"task-2","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"type":"회의","content":"회의 정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
			case request.Method == http.MethodPut:
				putCalled = true
				return flowToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.update",
		Input:    []byte(`{"query":"회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_task_ambiguous" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if putCalled {
		t.Fatal("ambiguous update must not write")
	}
	if !strings.Contains(string(response.Result), "task-1") || !strings.Contains(string(response.Result), "task-2") {
		t.Fatalf("result = %s", string(response.Result))
	}
}

func TestFlowTaskDeleteUsesSharedDeleteAPI(t *testing.T) {
	var deletedRequesterEmail string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/summary?week=26W24":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}],"weeklyTasks":[{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"business":"개발","type":"회의","content":"10분 회의","goal":"정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
			case request.Method == http.MethodDelete && request.URL.String() == "http://admind.local/flow/api/tasks/task-1":
				deletedRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
				return flowToolJSONResponse(`{"status":"deleted","task":{"id":"task-1"}}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.delete",
		Input:    []byte(`{"query":"10분 회의","weekCode":"26W24"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" {
		t.Fatalf("response = %+v", response)
	}
	if deletedRequesterEmail != "staff@example.com" {
		t.Fatalf("requester email = %q", deletedRequesterEmail)
	}
}

func TestNormalizeFlowStatusFilter(t *testing.T) {
	for input, expected := range map[string]string{
		"예약": "예정", "planned": "예정", "scheduled": "예정", "예정": "예정",
		"done": "완료", "완료": "완료", "in progress": "진행",
		"": "", "임의값": "임의값",
	} {
		if normalized := normalizeFlowStatusFilter(input); normalized != expected {
			t.Fatalf("normalizeFlowStatusFilter(%q) = %q, want %q", input, normalized, expected)
		}
	}
}

func flowTaskListTwoOwnerStateService(t *testing.T) Service {
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://admind.local/flow/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return flowToolJSONResponse(`{"currentWeek":{"code":"26W25"},"members":[{"id":"rain","name":"김테스트","email":"rain@example.com"},{"id":"lee","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"rain-future","ownerID":"rain","ownerName":"김테스트","content":"김테스트 예정 업무","status":"예정","weekCode":"26W30"},{"id":"rain-done","ownerID":"rain","ownerName":"김테스트","content":"김테스트 완료 업무","status":"완료","weekCode":"26W25"},{"id":"lee-task","ownerID":"lee","ownerName":"이샘플","content":"이샘플 업무","status":"예정","weekCode":"26W25"}]}`), nil
		})},
	}
}

func TestFlowTaskListDefaultsToRequesterOwnTasks(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.list",
		Input:    []byte(`{}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "rain-done") || strings.Contains(result, "lee-task") {
		t.Fatalf("expected only the requester's own tasks, got %s", result)
	}
	if !strings.Contains(result, `"scope":"self"`) || !strings.Contains(result, `"crossPerson":false`) || !strings.Contains(result, `"ownerID":"rain"`) {
		t.Fatalf("expected self scope metadata, got %s", result)
	}
}

func TestFlowTaskListSpansAllWeeksAndNormalizesStatusForOwnTasks(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.list",
		Input:    []byte(`{"status":"예약"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || strings.Contains(result, "rain-done") || strings.Contains(result, "lee-task") {
		t.Fatalf("expected only own planned task across weeks, got %s", result)
	}
	if !strings.Contains(result, `"statusFilter":"예정"`) {
		t.Fatalf("expected status normalized to 예정, got %s", result)
	}
}

func TestFlowTaskListAllPeopleReturnsEveryone(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.list",
		Input:    []byte(`{"allPeople":true}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "lee-task") {
		t.Fatalf("expected all owners' tasks, got %s", result)
	}
	if !strings.Contains(result, `"scope":"allPeople"`) || !strings.Contains(result, `"crossPerson":true`) {
		t.Fatalf("expected allPeople scope metadata, got %s", result)
	}
}

func TestFlowTaskListTargetPersonHintReturnsThatPerson(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "flow.task.list",
		Input:    []byte(`{"targetPersonHint":"이샘플"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "lee-task") || strings.Contains(result, "rain-future") {
		t.Fatalf("expected only the named person's tasks, got %s", result)
	}
	if !strings.Contains(result, `"scope":"person"`) {
		t.Fatalf("expected person scope metadata, got %s", result)
	}
}

func flowToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
