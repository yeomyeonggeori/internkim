package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
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
	resolution := resolveFlowOwner(flowTaskAddInput{Title: "10분 회의"}, "iam@example.com", members)
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
			resolution := resolveFlowOwner(flowTaskAddInput{Title: "업무 요청", TargetPersonHint: testCase.value}, "", members)
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
	resolution := resolveFlowOwner(flowTaskAddInput{Title: "10분 회의"}, "staff@example.com", members)
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

func TestResolveFlowOwnerReturnsNotFound(t *testing.T) {
	members := []flowMemberForTool{{ID: "lee", Name: "이동하", Email: "lee@example.com"}}
	resolution := resolveFlowOwner(flowTaskAddInput{TargetPersonHint: "Expensive"}, "lee@example.com", members)

	if resolution.OwnerID != "" || resolution.Failure == nil || resolution.Failure.ErrorCode != "flow_owner_not_found" {
		t.Fatalf("resolution = %+v", resolution)
	}
}

func TestResolveFlowOwnerRejectsContainedNameMatch(t *testing.T) {
	members := []flowMemberForTool{{ID: "kim", Name: "김인턴", Email: "kim@example.com", MattermostUsername: "internkim"}}
	resolution := resolveFlowOwner(flowTaskAddInput{TargetPersonHint: "인턴"}, "", members)

	if resolution.OwnerID != "" || resolution.Failure == nil || resolution.Failure.ErrorCode != "flow_owner_not_found" {
		t.Fatalf("resolution = %+v", resolution)
	}
}

func TestResolveFlowParticipantIDsUsesSharedPersonHints(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "owner", Name: "Owner", Email: "owner@example.com", MattermostUsername: "owner"},
		{ID: "kim", Name: "김인턴", Email: "kim@example.com", MattermostUsername: "internkim"},
	}
	participantIDs, failure := resolveFlowParticipantIDs([]string{"@internkim", "kim@example.com", "김인턴"}, "owner", members)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(participantIDs) != 2 || participantIDs[0] != "owner" || participantIDs[1] != "kim" {
		t.Fatalf("participantIDs = %+v", participantIDs)
	}
}

func TestResolveFlowParticipantIDsRejectsContainedNameMatch(t *testing.T) {
	members := []flowMemberForTool{{ID: "kim", Name: "김인턴", Email: "kim@example.com", MattermostUsername: "internkim"}}
	participantIDs, failure := resolveFlowParticipantIDs([]string{"인턴"}, "owner", members)
	if participantIDs != nil || failure == nil || failure.ErrorCode != "flow_participant_not_found" {
		t.Fatalf("participantIDs=%+v failure=%+v", participantIDs, failure)
	}
}

func TestResolveFlowParticipantIDsReturnsTypedAmbiguity(t *testing.T) {
	members := []flowMemberForTool{
		{ID: "lee", Name: "동하", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "동하", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	participantIDs, failure := resolveFlowParticipantIDs([]string{"동하"}, "owner", members)
	if participantIDs != nil || failure == nil || failure.ErrorCode != "flow_participant_ambiguous" || len(failure.Candidates) != 2 {
		t.Fatalf("participantIDs=%+v failure=%+v", participantIDs, failure)
	}
}

func TestFlowTaskAddPropagatesRequesterEmail(t *testing.T) {
	var summaryRequesterEmail string
	var taskRequesterEmail string
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				summaryRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks":
				taskRequesterEmail = request.Header.Get(flowRequesterEmailHeader)
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
		Input:    []byte(`{"title":"10분 회의"}`),
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

func TestFlowTaskWritesPreserveCallerContext(t *testing.T) {
	testCases := []struct {
		name   string
		invoke func(Service, context.Context) error
	}{
		{name: "add", invoke: func(service Service, requestContext context.Context) error {
			_, errorValue := service.postFlowTask(requestContext, flowTaskCreatePayload{OwnerID: "staff", ParticipantIDs: []string{"staff"}, Content: "업무"}, "staff@example.com")
			return errorValue
		}},
		{name: "update", invoke: func(service Service, requestContext context.Context) error {
			_, errorValue := service.putFlowTask(requestContext, flowTaskForTool{ID: "task-1"}, "staff@example.com")
			return errorValue
		}},
		{name: "delete", invoke: func(service Service, requestContext context.Context) error {
			_, errorValue := service.deleteFlowTask(requestContext, "task-1", "staff@example.com")
			return errorValue
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestStarted := make(chan bool, 1)
			service := Service{
				Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					_, hasDeadline := request.Context().Deadline()
					requestStarted <- hasDeadline
					<-request.Context().Done()
					return nil, request.Context().Err()
				})},
			}
			requestContext, cancelRequest := context.WithCancel(context.Background())
			result := make(chan error, 1)
			go func() {
				result <- testCase.invoke(service, requestContext)
			}()

			select {
			case hasDeadline := <-requestStarted:
				if hasDeadline {
					t.Fatal("flow task request added a child deadline")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("flow task request did not reach transport")
			}
			cancelRequest()
			select {
			case errorValue := <-result:
				if !errors.Is(errorValue, context.Canceled) {
					t.Fatalf("error = %v", errorValue)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("caller cancellation did not stop flow task request")
			}
		})
	}
}

func TestFlowTaskAddPropagatesTypedFields(t *testing.T) {
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"},{"id":"kim","name":"김인턴","email":"kim@example.com","mattermostUsername":"internkim"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks":
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
		Input:    []byte(`{"title":" 고객지원 분기 결산 누락 항목 확인 ","goal":" 누락 항목 확인 ","size":" s ","status":"예정","startDate":" 2026-07-15 ","endDate":" 2026-07-17 ","participantPersonHints":["@internkim"]}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "staff@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload["content"] != "고객지원 분기 결산 누락 항목 확인" {
		t.Fatalf("content = %#v", payload["content"])
	}
	if payload["endDate"] != "2026-07-17" {
		t.Fatalf("endDate = %#v", payload["endDate"])
	}
	if payload["goal"] != "누락 항목 확인" || payload["size"] != "S" || payload["status"] != "예정" || payload["startDate"] != "2026-07-15" {
		t.Fatalf("payload = %#v", payload)
	}
	for _, fieldName := range []string{"prompt", "title", "targetPersonHint", "participantPersonHints", "requesterEmail", "source", "weekCode", "allowDuplicate", "duplicatePolicy"} {
		if _, found := payload[fieldName]; found {
			t.Fatalf("unexpected payload field %q in %#v", fieldName, payload)
		}
	}
	participantIDs, ok := payload["participantIDs"].([]any)
	if !ok || len(participantIDs) != 2 || participantIDs[0] != "staff" || participantIDs[1] != "kim" {
		t.Fatalf("participantIDs = %#v", payload["participantIDs"])
	}
}

func TestDecodeFlowTaskAddInputRejectsLegacyPromptAndContent(t *testing.T) {
	for _, document := range []string{`{"prompt":"업무 추가"}`, `{"content":"업무 추가"}`} {
		if _, errorValue := decodeFlowTaskAddInput([]byte(document)); errorValue == nil || !strings.Contains(errorValue.Error(), "unknown field") {
			t.Fatalf("error = %v for %s", errorValue, document)
		}
	}
}

func TestDecodeFlowTaskAddInputTrimsCanonicalFields(t *testing.T) {
	input, errorValue := decodeFlowTaskAddInput([]byte(`{"title":" 정확한 제목 ","goal":" 목표 ","size":" m ","status":" 진행 ","startDate":" 2026-07-15 ","endDate":" 2026-07-17 ","targetPersonHint":" @lee ","participantPersonHints":[" @kim ","@kim"," "]}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.Title != "정확한 제목" || input.Goal != "목표" || input.Size != "M" || input.Status != "진행" || input.StartDate != "2026-07-15" || input.EndDate != "2026-07-17" || input.TargetPersonHint != "@lee" || len(input.ParticipantPersonHints) != 1 || input.ParticipantPersonHints[0] != "@kim" {
		t.Fatalf("input = %+v", input)
	}
}

func TestDecodeFlowTaskAddInputRejectsInvalidTypedFields(t *testing.T) {
	for _, testCase := range []struct {
		document     string
		errorMessage string
	}{
		{document: `{}`, errorMessage: "title is required"},
		{document: `{"title":"업무","size":"HUGE"}`, errorMessage: "size is not allowed"},
		{document: `{"title":"업무","status":"unknown"}`, errorMessage: "status is not allowed"},
		{document: `{"title":"업무","extra":true}`, errorMessage: "unknown field"},
		{document: `{"title":"업무","weekCode":"26W29"}`, errorMessage: "unknown field"},
		{document: `{"title":"업무","allowDuplicate":true}`, errorMessage: "unknown field"},
	} {
		_, errorValue := decodeFlowTaskAddInput([]byte(testCase.document))
		if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.errorMessage) {
			t.Fatalf("error = %v for %s", errorValue, testCase.document)
		}
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
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks":
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
		Input:    []byte(`{"title":"업무 요청","targetPersonHint":"동하"}`),
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

func TestFlowTaskAddReturnsParticipantResolutionErrorBeforeCreate(t *testing.T) {
	postCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"},{"id":"lee","name":"동하","email":"lee@example.com"},{"id":"kim","name":"동하","email":"kim@example.com"}]}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks":
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
		Input:    []byte(`{"title":"공동 업무","participantPersonHints":["동하"]}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_participant_ambiguous" || postCalled {
		t.Fatalf("response=%+v postCalled=%t", response, postCalled)
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
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks":
				return flowToolJSONResponse(`{"id":"task-1","participantIDs":["rain"],"participantNames":["신우경"],"content":"경산 일정","status":"진행"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.add",
		Input:    []byte(`{"title":"경산 일정"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		TaskID                   string                      `json:"taskID"`
		ParticipantPresentations []personPresentationForTool `json:"participantPresentations"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.TaskID != "task-1" || len(result.ParticipantPresentations) != 1 || result.ParticipantPresentations[0].MattermostMention != "@rain" {
		t.Fatalf("participant presentations = %+v", result.ParticipantPresentations)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "task-1" || response.Effects[0].Effect != "created" {
		t.Fatalf("task.add effects = %+v", response)
	}
}

func TestFlowTaskAddReportsDuplicateAsTypedFailure(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet {
				return flowToolJSONResponse(`{"members":[{"id":"staff","name":"Staff","email":"staff@example.com"}]}`), nil
			}
			return flowToolJSONResponse(`{"status":"skipped_duplicate","duplicateTask":{"id":"task-existing"}}`), nil
		})},
	}

	response, errorValue := service.invokeFlowTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.add",
		Input:    []byte(`{"title":"고객지원 분기 결산"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "flow_task_duplicate" || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
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
			case request.Method == http.MethodPost && request.URL.String() == "http://admind.local/flow/api/tasks":
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
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "task-1" || response.Effects[0].Effect != "updated" {
		t.Fatalf("task.update effects = %+v", response)
	}
}

func TestFlowTaskUpdateRejectsMismatchedBackendTaskID(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet {
				return flowToolJSONResponse(`{"tasks":[{"id":"task-1","content":"이전 업무"}]}`), nil
			}
			return flowToolJSONResponse(`{"id":"task-2","content":"수정 업무"}`), nil
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.update",
		Input:    []byte(`{"taskID":"task-1","title":"수정 업무"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "flow_task_result_invalid" || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestFlowTaskUpdateRejectsBackendNoOp(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Method == http.MethodGet {
				return flowToolJSONResponse(`{"tasks":[{"id":"task-1","content":"이전 업무"}]}`), nil
			}
			return flowToolJSONResponse(`{"id":"task-1","content":"이전 업무"}`), nil
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.update",
		Input:    []byte(`{"taskID":"task-1","title":"수정 업무"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "flow_task_result_invalid" || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestDecodeFlowTaskUpdateInputRejectsLegacyAndResolutionFields(t *testing.T) {
	for _, document := range []string{
		`{"taskID":"task-1","content":"이전 제목"}`,
		`{"taskID":"task-1","query":"이전 제목","status":"완료"}`,
		`{"taskID":"task-1","targetPersonHint":"staff","status":"완료"}`,
		`{"taskID":"task-1","weekCode":"26W24","status":"완료"}`,
	} {
		_, errorValue := decodeFlowTaskUpdateInput([]byte(document))
		if errorValue == nil || !strings.Contains(errorValue.Error(), "unknown field") {
			t.Fatalf("document = %s error = %v", document, errorValue)
		}
	}
}

func TestDecodeFlowTaskUpdateInputRequiresTaskIDAndPatch(t *testing.T) {
	for _, document := range []string{
		`{"status":"완료"}`,
		`{"taskID":"task-1"}`,
		`{"taskID":" "}`,
	} {
		if _, errorValue := decodeFlowTaskUpdateInput([]byte(document)); errorValue == nil {
			t.Fatalf("document = %s expected validation error", document)
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
		Input:    []byte(`{"taskID":"deck-1","status":"완료"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@dawn.kim"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected exact taskID to resolve, got error response %+v", response)
	}
	if updatedPayload["status"] != "완료" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestFlowTaskUpdateMissingExactIDReturnsTypedNotFound(t *testing.T) {
	putCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://admind.local/flow/api/state":
				return flowToolJSONResponse(`{"members":[],"tasks":[{"id":"task-1","content":"회의","status":"진행"}]}`), nil
			case request.Method == http.MethodPut:
				putCalled = true
				return flowToolJSONResponse(`{"id":"missing-task","status":"완료"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeFlowTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.update",
		Input:    []byte(`{"taskID":"missing-task","status":"완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_task_not_found" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if putCalled {
		t.Fatal("missing exact taskID must not write")
	}
}

func TestFlowTaskDeleteUsesSharedDeleteAPI(t *testing.T) {
	var deletedRequesterEmail string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			switch {
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
		Input:    []byte(`{"taskID":"task-1"}`),
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
	if string(response.Result) != `{"deleted":true,"taskID":"task-1"}` {
		t.Fatalf("result = %s", response.Result)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "task-1" || response.Effects[0].Effect != "deleted" {
		t.Fatalf("task.delete effects = %+v", response)
	}
	if deletedRequesterEmail != "staff@example.com" {
		t.Fatalf("requester email = %q", deletedRequesterEmail)
	}
}

func TestDecodeFlowTaskDeleteInputRejectsResolutionFields(t *testing.T) {
	for _, document := range []string{
		`{"query":"회의"}`,
		`{"taskID":"task-1","targetPersonHint":"staff"}`,
		`{"taskID":"task-1","weekCode":"26W24"}`,
	} {
		_, errorValue := decodeFlowTaskDeleteInput([]byte(document))
		if errorValue == nil {
			t.Fatalf("document = %s expected validation error", document)
		}
	}
}

func TestFlowTaskDeleteNotFoundReturnsTypedFailure(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			response := flowToolJSONResponse(`{"error":"task not found"}`)
			response.StatusCode = http.StatusNotFound
			return response, nil
		})},
	}

	response, errorValue := service.invokeFlowTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.delete",
		Input:    []byte(`{"taskID":"missing-task"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_task_not_found" || response.Status == "deleted" {
		t.Fatalf("response = %+v", response)
	}
}

func TestFlowTaskDeleteReturnsTypedNotFoundWithoutExactEvidence(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return flowToolJSONResponse(`{"status":"deleted","task":{"id":"other-task"}}`), nil
		})},
	}

	response, errorValue := service.invokeFlowTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.delete",
		Input:    []byte(`{"taskID":"task-1"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_task_not_found" || response.Status == "deleted" {
		t.Fatalf("response = %+v error = %v", response, errorValue)
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

func TestDecodeFlowTaskListInputRejectsTaskAddFields(t *testing.T) {
	_, errorValue := decodeFlowTaskListInput([]byte(`{"prompt":"업무 추가","title":"분기 결산","endDate":"2026-07-17"}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "unknown field") {
		t.Fatalf("expected task.add fields to fail task.list validation, got %v", errorValue)
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

func TestFlowTaskListUnknownPersonDoesNotListEveryone(t *testing.T) {
	service := flowTaskListTwoOwnerStateService(t)
	response, errorValue := service.invokeFlowTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task.list",
		Input:    []byte(`{"targetPersonHint":"Expensive","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "flow_owner_not_found" {
		t.Fatalf("expected unresolved person error, got %+v", response)
	}
	if strings.Contains(string(response.Result), "rain-future") || strings.Contains(string(response.Result), "lee-task") {
		t.Fatalf("expected no task disclosure, got %s", response.Result)
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
	single, errorValue := flowTaskListWeekCodes(0, 0, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(single) != 1 || !single[thisWeek] {
		t.Fatalf("expected only this week %q, got %v", thisWeek, single)
	}
	threeWeeks, errorValue := flowTaskListWeekCodes(-2, 0, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(threeWeeks) != 3 || !threeWeeks[thisWeek] ||
		!threeWeeks[weekCodeForFlowDate(now.AddDate(0, 0, -7))] ||
		!threeWeeks[weekCodeForFlowDate(now.AddDate(0, 0, -14))] {
		t.Fatalf("expected this and the prior two weeks, got %v", threeWeeks)
	}
	swappedWeeks, errorValue := flowTaskListWeekCodes(0, -1, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(swappedWeeks) != 2 {
		t.Fatalf("expected swapped bounds to span 2 weeks")
	}
	wideWeeks, errorValue := flowTaskListWeekCodes(-1000, 0, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if wideWeeks != nil {
		t.Fatalf("expected no week filter for a very wide range")
	}
	if _, errorValue := flowTaskListWeekCodes(0, 0, "invalid"); errorValue == nil {
		t.Fatal("expected an invalid canonical week to fail closed")
	}
}

func TestFlowTaskListDefaultsToThisWeekOnly(t *testing.T) {
	thisWeek := "26W30"
	priorWeek := "26W29"
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
