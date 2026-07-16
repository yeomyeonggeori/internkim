package capabilityd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestResolveFlowOwnerUsesRequesterWhenTargetPersonHintIsEmpty(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "lee", Email: "lee@example.com"},
		{ID: "iam", Name: "iam", Email: "iam@example.com"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{Prompt: "lee에게 10분 회의 추가해줘"}, "iam@example.com", members)
	if resolution.OwnerID != "iam" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveFlowOwnerMatchesMattermostHandle(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "Lee Dongha", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "Kim Dongha", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{TargetPersonHint: "@kim"}, "", members)
	if resolution.OwnerID != "kim" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveFlowOwnerMatchesTargetPersonHint(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee-1", Name: "이동하", Email: "lee@example.com", MattermostUsername: "lee"},
	}
	for _, testCase := range []struct {
		name  string
		value string
	}{
		{name: "handle", value: "@lee"},
		{name: "name", value: "이동하"},
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
		{ID: "lee", Name: "동하", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "동하", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	resolution := resolveFlowOwner(flowTaskAddInput{TargetPersonHint: "동하"}, "", members)
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
		ToolName: "task.add",
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

func TestFlowTaskAddPropagatesExplicitTitleEndDateAndDuplicateConfirmation(t *testing.T) {
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
		ToolName: "task.add",
		Input:    []byte(`{"prompt":"고객지원팀의 분기 결산 자료에서 누락 항목을 확인하는 업무","title":" 고객지원 분기 결산 누락 항목 확인 ","endDate":" 2026-07-17 ","allowDuplicate":true}`),
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
	if payload["title"] != "고객지원 분기 결산 누락 항목 확인" {
		t.Fatalf("title = %#v", payload["title"])
	}
	if payload["endDate"] != "2026-07-17" {
		t.Fatalf("endDate = %#v", payload["endDate"])
	}
}

func TestDecodeFlowTaskAddInputAcceptsLegacyContentAlias(t *testing.T) {
	input, errorValue := decodeFlowTaskAddInput([]byte(`{"prompt":"업무 추가","content":" 이전 제목 "}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.Title != "이전 제목" {
		t.Fatalf("title = %q", input.Title)
	}
}

func TestDecodeFlowTaskAddInputTrimsCanonicalFields(t *testing.T) {
	input, errorValue := decodeFlowTaskAddInput([]byte(`{"prompt":" 업무 추가 ","title":" 정확한 제목 ","endDate":" 2026-07-17 "}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.Prompt != "업무 추가" || input.Title != "정확한 제목" || input.EndDate != "2026-07-17" {
		t.Fatalf("input = %+v", input)
	}
}

func TestFlowTaskAddReturnsAmbiguousOwnerError(t *testing.T) {
	postCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"lee","name":"동하","email":"lee@example.com","mattermostUsername":"lee"},{"id":"kim","name":"동하","email":"kim@example.com","mattermostUsername":"kim"}]}`), nil
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
		ToolName: "task.add",
		Input:    []byte(`{"prompt":"업무 요청해줘","targetPersonHint":"동하"}`),
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
		ToolName: "task.add",
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
		ToolName: "task.list",
		Input:    []byte(`{"query":"디플랫 코리아","weekFrom":-1000}`),
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

func TestFlowTaskAddAddsParticipantPresentations(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"rain","name":"신우경","email":"rain@example.com","mattermostUsername":"rain"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks/quick":
				return flowToolJSONResponse(`{"id":"task-1","participantIDs":["rain"],"participantNames":["신우경"],"content":"경산 일정","status":"진행"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.add",
		Input:    []byte(`{"prompt":"경산 일정"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		ParticipantPresentations []personPresentationForTool `json:"participantPresentations"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(result.ParticipantPresentations) != 1 || result.ParticipantPresentations[0].MattermostMention != "@rain" {
		t.Fatalf("participant presentations = %+v", result.ParticipantPresentations)
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
				return flowToolJSONResponse(`{"members":[{"id":"foreign","name":"Foreign","email":"foreign@example.com"},{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"foreign-task","ownerID":"foreign","ownerName":"Foreign","participantIDs":["foreign"],"content":"10분 회의","status":"진행","weekCode":"26W24"},{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"business":"개발","type":"회의","content":"10분 회의","goal":"정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
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
		ToolName: "task.update",
		Input:    []byte(`{"taskID":"task-1","title":"15분 회의"}`),
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

func TestDecodeFlowTaskUpdateInputAcceptsLegacyContentAlias(t *testing.T) {
	input, errorValue := decodeFlowTaskUpdateInput([]byte(`{"taskID":"task-1","content":" 이전 제목 "}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.Title == nil || *input.Title != "이전 제목" {
		t.Fatalf("title = %#v", input.Title)
	}
}

func TestDecodeFlowTaskInputRejectsTitleAndLegacyContentConflict(t *testing.T) {
	testCases := []struct {
		document string
		decode   func([]byte) error
	}{
		{document: `{"prompt":"업무 추가","title":"같은 제목","content":"같은 제목"}`, decode: func(document []byte) error {
			_, errorValue := decodeFlowTaskAddInput(document)
			return errorValue
		}},
		{document: `{"taskID":"task-1","title":"새 제목","content":"이전 제목"}`, decode: func(document []byte) error {
			_, errorValue := decodeFlowTaskUpdateInput(document)
			return errorValue
		}},
	}
	for _, testCase := range testCases {
		errorValue := testCase.decode([]byte(testCase.document))
		if errorValue == nil || !strings.Contains(errorValue.Error(), "both title and content") {
			t.Fatalf("error = %v for %s", errorValue, testCase.document)
		}
	}
}

func TestDecodeFlowTaskUpdateInputTrimsTitle(t *testing.T) {
	input, errorValue := decodeFlowTaskUpdateInput([]byte(`{"taskID":" task-1 ","title":" 정확한 제목 "}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.TaskID != "task-1" || input.Title == nil || *input.Title != "정확한 제목" {
		t.Fatalf("input = %+v", input)
	}
}

func TestFlowTaskUpdateResolvesByTaskIDAcrossAllTasks(t *testing.T) {
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"이동하","email":"lee@dawn.kim"}],"tasks":[{"id":"deck-1","ownerID":"staff","ownerName":"이동하","participantIDs":["staff"],"business":"여명거리","type":"문서","content":"IR 덱","status":"진행","weekCode":"26W28"}]}`), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://admind.local/flow/api/tasks/deck-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return flowToolJSONResponse(`{"id":"deck-1","status":"완료"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.update",
		Input:    []byte(`{"taskID":"deck-1","status":"완료","targetPersonHint":"","weekCode":"2026-W28"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected taskID to resolve regardless of week/person hint, got error response %+v", response)
	}
	if updatedPayload["status"] != "완료" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestFlowTaskUpdateQueryOnlyDefaultsToComplete(t *testing.T) {
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"foreign","name":"Foreign","email":"foreign@example.com"},{"id":"staff","name":"Staff","email":"staff@example.com"}],"tasks":[{"id":"foreign-task","ownerID":"foreign","ownerName":"Foreign","participantIDs":["foreign"],"content":"10분 회의","status":"진행","weekCode":"26W24"},{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"business":"개발","type":"회의","content":"10분 회의","goal":"정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
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
		ToolName: "task.update",
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
		ToolName: "task.update",
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
				return flowToolJSONResponse(`{"members":[{"id":"foreign","name":"Foreign","email":"foreign@example.com"},{"id":"staff","name":"Staff","email":"staff@example.com"}],"weeklyTasks":[{"id":"foreign-task","ownerID":"foreign","ownerName":"Foreign","participantIDs":["foreign"],"content":"10분 회의","status":"진행","weekCode":"26W24"},{"id":"task-1","ownerID":"staff","ownerName":"Staff","participantIDs":["staff"],"participantNames":["Staff"],"business":"개발","type":"회의","content":"10분 회의","goal":"정리","size":"XS","status":"진행","weekCode":"26W24"}]}`), nil
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
		ToolName: "task.delete",
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
			return flowToolJSONResponse(`{"currentWeek":{"code":"26W25"},"members":[{"id":"rain","name":"신우경","email":"rain@example.com"},{"id":"lee","name":"이동하","email":"lee@example.com"}],"tasks":[{"id":"rain-future","ownerID":"rain","ownerName":"신우경","content":"신우경 예정 업무","status":"예정","weekCode":"26W30"},{"id":"rain-done","ownerID":"rain","ownerName":"신우경","content":"신우경 완료 업무","status":"완료","weekCode":"26W25"},{"id":"lee-task","ownerID":"lee","ownerName":"이동하","content":"이동하 업무","status":"예정","weekCode":"26W25"}]}`), nil
		})},
	}
}

func TestFlowTaskListEmptyHintDefaultsToRequester(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "rain-done") || strings.Contains(result, "lee-task") {
		t.Fatalf("expected requester tasks when no person is named, got %s", result)
	}
	if !strings.Contains(result, `"scope":"person"`) || !strings.Contains(result, `"ownerID":"rain"`) {
		t.Fatalf("expected requester scope metadata, got %s", result)
	}
}

func TestFlowTaskListAllScopeListsEveryone(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"scope":"all","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "rain-done") || !strings.Contains(result, "lee-task") {
		t.Fatalf("expected all tasks for explicit all scope, got %s", result)
	}
	if !strings.Contains(result, `"scope":"everyone"`) || !strings.Contains(result, `"ownerID":""`) {
		t.Fatalf("expected everyone scope metadata, got %s", result)
	}
}

func TestFlowTaskListOwnNameNarrowsToRequester(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"targetPersonHint":"신우경","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "rain-done") || strings.Contains(result, "lee-task") {
		t.Fatalf("expected only the named requester's own tasks, got %s", result)
	}
	if !strings.Contains(result, `"scope":"person"`) || !strings.Contains(result, `"ownerID":"rain"`) {
		t.Fatalf("expected person scope for own name, got %s", result)
	}
}

func TestFlowTaskListNormalizesStatusAcrossEveryone(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"scope":"all","status":"예약","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "lee-task") || strings.Contains(result, "rain-done") {
		t.Fatalf("expected everyone's planned tasks across weeks, got %s", result)
	}
	if !strings.Contains(result, `"statusFilter":"예정"`) {
		t.Fatalf("expected status normalized to 예정, got %s", result)
	}
}

func TestFlowTaskListTargetPersonHintReturnsThatPerson(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"targetPersonHint":"이동하","weekFrom":-1000}`),
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

func TestFlowTaskListTargetPersonHintIncludesParticipantTasks(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://admind.local/flow/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return flowToolJSONResponse(`{"currentWeek":{"code":"26W25"},"members":[{"id":"owner","name":"오너","email":"owner@example.com"},{"id":"lee","name":"이동하","email":"lee@example.com"}],"tasks":[{"id":"owner-task","ownerID":"owner","ownerName":"오너","participantIDs":["owner","lee"],"participantNames":["오너","이동하"],"content":"이동하 참여 업무","status":"진행","weekCode":"26W25"},{"id":"other-task","ownerID":"owner","ownerName":"오너","participantIDs":["owner"],"participantNames":["오너"],"content":"오너 단독 업무","status":"진행","weekCode":"26W25"}]}`), nil
		})},
	}
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"targetPersonHint":"이동하","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "owner@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "owner-task") || strings.Contains(result, "other-task") {
		t.Fatalf("expected tasks where the named person participates, got %s", result)
	}
}

func TestFlowTaskListTreatsAvailablePlannedAndPausedAsCurrentWeek(t *testing.T) {
	now := time.Now()
	thisWeek := weekCodeForFlowDate(now)
	priorWeek := weekCodeForFlowDate(now.AddDate(0, 0, -14))
	futureStartDate := now.AddDate(0, 0, 14).Format("2006-01-02")
	stateBody := fmt.Sprintf(`{"currentWeek":{"code":%q},"members":[{"id":"lee","name":"이동하","email":"lee@example.com"}],"tasks":[{"id":"planned-old-week","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"예정 업무","status":"예정","weekCode":%q},{"id":"planned-future-start","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"미래 예정 업무","status":"예정","startDate":%q,"weekCode":%q},{"id":"paused-old-week","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"일시정지 업무","status":"일시정지","weekCode":%q}]}`, thisWeek, priorWeek, futureStartDate, priorWeek, priorWeek)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://admind.local/flow/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return flowToolJSONResponse(stateBody), nil
		})},
	}
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"targetPersonHint":"이동하"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "planned-old-week") || !strings.Contains(result, "paused-old-week") {
		t.Fatalf("expected planned and paused tasks to be treated as current week, got %s", result)
	}
	if strings.Contains(result, "planned-future-start") {
		t.Fatalf("expected future planned tasks to stay out of current week, got %s", result)
	}
}

func TestFlowTaskListClassifiesFinishedInactiveTasksByDates(t *testing.T) {
	now := time.Now()
	thisWeek := weekCodeForFlowDate(now)
	oldWeek := weekCodeForFlowDate(now.AddDate(0, 0, -28))
	thisWeekDate := now.Format("2006-01-02")
	priorWeekDate := now.AddDate(0, 0, -7).Format("2006-01-02")
	stateBody := fmt.Sprintf(`{"currentWeek":{"code":%q},"members":[{"id":"lee","name":"이동하","email":"lee@example.com"}],"tasks":[{"id":"completed-current-end","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"완료 업무","status":"완료","startDate":%q,"endDate":%q,"weekCode":%q},{"id":"rejected-current-end","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"기각 업무","status":"기각","startDate":%q,"endDate":%q,"weekCode":%q},{"id":"stopped-current-start","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"중단 시작일 업무","status":"중단","startDate":%q,"weekCode":%q},{"id":"stopped-prior-end","ownerID":"lee","ownerName":"이동하","participantIDs":["lee"],"participantNames":["이동하"],"content":"중단 종료일 우선 업무","status":"중단","startDate":%q,"endDate":%q,"weekCode":%q}]}`, thisWeek, priorWeekDate, thisWeekDate, oldWeek, priorWeekDate, thisWeekDate, oldWeek, thisWeekDate, oldWeek, thisWeekDate, priorWeekDate, oldWeek)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://admind.local/flow/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return flowToolJSONResponse(stateBody), nil
		})},
	}
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"targetPersonHint":"이동하"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	for _, expectedTaskID := range []string{"completed-current-end", "rejected-current-end", "stopped-current-start"} {
		if !strings.Contains(result, expectedTaskID) {
			t.Fatalf("expected %s in current week result, got %s", expectedTaskID, result)
		}
	}
	if strings.Contains(result, "stopped-prior-end") {
		t.Fatalf("expected end date to take precedence over start date, got %s", result)
	}
}

func TestFlowTaskListWeekCodes(t *testing.T) {
	now := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	thisWeek := weekCodeForFlowDate(now)
	single := flowTaskListWeekCodes(0, 0, now)
	if len(single) != 1 || !single[thisWeek] {
		t.Fatalf("expected only this week %q, got %v", thisWeek, single)
	}
	threeWeeks := flowTaskListWeekCodes(-2, 0, now)
	if len(threeWeeks) != 3 || !threeWeeks[thisWeek] ||
		!threeWeeks[weekCodeForFlowDate(now.AddDate(0, 0, -7))] ||
		!threeWeeks[weekCodeForFlowDate(now.AddDate(0, 0, -14))] {
		t.Fatalf("expected this and the prior two weeks, got %v", threeWeeks)
	}
	if len(flowTaskListWeekCodes(0, -1, now)) != 2 {
		t.Fatalf("expected swapped bounds to span 2 weeks")
	}
	if flowTaskListWeekCodes(-1000, 0, now) != nil {
		t.Fatalf("expected no week filter for a very wide range")
	}
}

func TestFlowTaskListDefaultsToThisWeekOnly(t *testing.T) {
	thisWeek := weekCodeForFlowDate(time.Now())
	priorWeek := weekCodeForFlowDate(time.Now().AddDate(0, 0, -21))
	stateBody := fmt.Sprintf(`{"currentWeek":{"code":%q},"members":[{"id":"lee","name":"이동하","email":"lee@example.com"}],"tasks":[{"id":"this-week-task","ownerID":"lee","ownerName":"이동하","content":"이번주 업무","status":"진행","weekCode":%q},{"id":"prior-week-task","ownerID":"lee","ownerName":"이동하","content":"지난 업무","status":"진행","weekCode":%q}]}`, thisWeek, thisWeek, priorWeek)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return flowToolJSONResponse(stateBody), nil
		})},
	}
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "this-week-task") || strings.Contains(result, "prior-week-task") {
		t.Fatalf("expected only this week's task by default, got %s", result)
	}
}

func flowToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}
