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

func TestResolveTaskOwnerUsesRequesterWhenTargetPersonHintIsEmpty(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "lee", Name: "lee", Email: "lee@example.com"},
		{ID: "iam", Name: "iam", Email: "iam@example.com"},
	}
	resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), nil, "iam@example.com", members)
	if resolution.OwnerID != "iam" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveTaskOwnerMatchesMattermostHandle(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "lee", Name: "Lee Dongha", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "Kim Dongha", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), []string{"@kim"}, "", members)
	if resolution.OwnerID != "kim" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveTaskOwnerMatchesTargetPersonHint(t *testing.T) {
	members := []taskMemberForTool{
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
			resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), []string{testCase.value}, "", members)
			if resolution.OwnerID != "lee-1" {
				t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
			}
		})
	}
}

func TestResolveTaskOwnerFallsBackToRequesterEmail(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "member", Name: "Member", Email: "member@example.com", MattermostUsername: "member"},
	}
	resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), nil, "member@example.com", members)
	if resolution.OwnerID != "member" {
		t.Fatalf("ownerID = %q failure=%+v", resolution.OwnerID, resolution.Failure)
	}
}

func TestResolveTaskOwnerReturnsAmbiguousCandidates(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "lee", Name: "샘플", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "샘플", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), []string{"샘플"}, "", members)
	if resolution.Failure == nil || resolution.Failure.ErrorCode != "task_owner_ambiguous" {
		t.Fatalf("failure = %+v", resolution.Failure)
	}
	if len(resolution.Failure.Candidates) != 2 || resolution.Failure.Candidates[0].Mention != "@lee" || resolution.Failure.Candidates[1].Mention != "@kim" {
		t.Fatalf("candidates = %+v", resolution.Failure.Candidates)
	}
}

func TestResolveTaskOwnerReturnsNotFound(t *testing.T) {
	members := []taskMemberForTool{{ID: "lee", Name: "이샘플", Email: "lee@example.com"}}
	resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), []string{"Expensive"}, "lee@example.com", members)

	if resolution.OwnerID != "" || resolution.Failure == nil || resolution.Failure.ErrorCode != "task_owner_not_found" {
		t.Fatalf("resolution = %+v", resolution)
	}
}

func sampleTaskMembers() []taskMemberForTool {
	return []taskMemberForTool{
		{ID: "person-sample", Name: "이샘플", Email: "sample@example.com", MattermostUsername: "sampleuser"},
		{ID: "person-specimen", Name: "최견본", Email: "specimen@example.com", MattermostUsername: "specimenuser"},
	}
}

func TestResolveTaskOwnerAcceptsUniqueNameFragment(t *testing.T) {
	resolution := serviceWithDirectoryOf(t, sampleTaskMembers()).resolveTaskOwner(context.Background(), []string{"견본"}, "", sampleTaskMembers())

	if resolution.OwnerID != "person-specimen" {
		t.Fatalf("resolution = %+v", resolution)
	}
}

func TestResolveTaskOwnerOffersCandidatesForAmbiguousNameFragment(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "person-example", Name: "박예시", Email: "example@example.com", MattermostUsername: "exampleuser"},
		{ID: "person-other-example", Name: "이예시", Email: "other@example.com", MattermostUsername: "otheruser"},
	}
	resolution := serviceWithDirectoryOf(t, members).resolveTaskOwner(context.Background(), []string{"예시"}, "", members)

	if resolution.Failure == nil || resolution.Failure.ErrorCode != "task_owner_ambiguous" || len(resolution.Failure.Candidates) != 2 {
		t.Fatalf("resolution = %+v", resolution)
	}
}

func TestResolveTaskOwnerRejectsHintContainingTheName(t *testing.T) {
	resolution := serviceWithDirectoryOf(t, sampleTaskMembers()).resolveTaskOwner(context.Background(), []string{"최견본이랑 방금 운동함"}, "", sampleTaskMembers())

	if resolution.OwnerID != "" || resolution.Failure == nil || resolution.Failure.ErrorCode != "task_owner_not_found" {
		t.Fatalf("resolution = %+v", resolution)
	}
}

func TestResolveTaskOwnerKeepsHandleAndEmailExact(t *testing.T) {
	for _, personHint := range []string{"specimenuser", "specimen", "@specimen", "specimen@exam", "example.com"} {
		resolution := serviceWithDirectoryOf(t, sampleTaskMembers()).resolveTaskOwner(context.Background(), []string{personHint}, "", sampleTaskMembers())
		if resolution.OwnerID != "" || resolution.Failure == nil {
			t.Fatalf("personHint %q resolved to %+v", personHint, resolution)
		}
	}
}

func TestResolveTaskOwnerNotFoundListsTheRoster(t *testing.T) {
	resolution := serviceWithDirectoryOf(t, sampleTaskMembers()).resolveTaskOwner(context.Background(), []string{"박예시"}, "", sampleTaskMembers())

	if resolution.Failure == nil || resolution.Failure.ErrorCode != "task_owner_not_found" {
		t.Fatalf("resolution = %+v", resolution)
	}
	if len(resolution.Failure.Candidates) != 2 || resolution.Failure.Candidates[0].Mention != "@sampleuser" {
		t.Fatalf("candidates = %+v", resolution.Failure.Candidates)
	}
}

func TestResolveTaskParticipantIDsUsesSharedPersonHints(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "owner", Name: "Owner", Email: "owner@example.com", MattermostUsername: "owner"},
		{ID: "kim", Name: "김인턴", Email: "kim@example.com", MattermostUsername: "internkim"},
	}
	participantIDs, failure := serviceWithDirectoryOf(t, members).resolveTaskParticipantIDs(context.Background(), []string{"@internkim", "kim@example.com", "김인턴"}, "owner", members)
	if failure != nil {
		t.Fatal(failure)
	}
	if len(participantIDs) != 2 || participantIDs[0] != "owner" || participantIDs[1] != "kim" {
		t.Fatalf("participantIDs = %+v", participantIDs)
	}
}

func TestResolveTaskParticipantIDsAcceptsUniqueNameFragment(t *testing.T) {
	participantIDs, failure := serviceWithDirectoryOf(t, sampleTaskMembers()).resolveTaskParticipantIDs(context.Background(), []string{"견본"}, "person-sample", sampleTaskMembers())
	if failure != nil {
		t.Fatal(failure)
	}
	if len(participantIDs) != 2 || participantIDs[1] != "person-specimen" {
		t.Fatalf("participantIDs = %+v", participantIDs)
	}
}

func TestResolveTaskParticipantIDsNotFoundListsTheRoster(t *testing.T) {
	participantIDs, failure := serviceWithDirectoryOf(t, sampleTaskMembers()).resolveTaskParticipantIDs(context.Background(), []string{"박예시"}, "person-sample", sampleTaskMembers())
	if participantIDs != nil || failure == nil || failure.ErrorCode != "task_participant_not_found" {
		t.Fatalf("participantIDs=%+v failure=%+v", participantIDs, failure)
	}
	if len(failure.Candidates) != 2 || failure.Candidates[0].Mention != "@sampleuser" {
		t.Fatalf("candidates = %+v", failure.Candidates)
	}
}

func TestResolveTaskParticipantIDsReturnsTypedAmbiguity(t *testing.T) {
	members := []taskMemberForTool{
		{ID: "lee", Name: "샘플", Email: "lee@example.com", MattermostUsername: "lee"},
		{ID: "kim", Name: "샘플", Email: "kim@example.com", MattermostUsername: "kim"},
	}
	participantIDs, failure := serviceWithDirectoryOf(t, members).resolveTaskParticipantIDs(context.Background(), []string{"샘플"}, "owner", members)
	if participantIDs != nil || failure == nil || failure.ErrorCode != "task_participant_ambiguous" || len(failure.Candidates) != 2 {
		t.Fatalf("participantIDs=%+v failure=%+v", participantIDs, failure)
	}
}

func TestTaskAddPropagatesRequesterEmail(t *testing.T) {
	var summaryRequesterEmail string
	var taskRequesterEmail string
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				summaryRequesterEmail = request.Header.Get(admindRequesterEmailHeader)
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"Member","email":"member@example.com"}]}`)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				taskRequesterEmail = request.Header.Get(admindRequesterEmailHeader)
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return taskToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"10분 회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "member@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if summaryRequesterEmail != "member@example.com" || taskRequesterEmail != "member@example.com" {
		t.Fatalf("requester headers summary=%q task=%q", summaryRequesterEmail, taskRequesterEmail)
	}
}

func TestTaskWritesPreserveCallerContext(t *testing.T) {
	testCases := []struct {
		name   string
		invoke func(Service, context.Context) error
	}{
		{name: "add", invoke: func(service Service, requestContext context.Context) error {
			_, errorValue := service.postTask(requestContext, taskCreatePayload{OwnerID: "member", ParticipantIDs: []string{"member"}, Content: "업무"}, "member@example.com")
			return errorValue
		}},
		{name: "update", invoke: func(service Service, requestContext context.Context) error {
			_, errorValue := service.putTask(requestContext, taskForTool{ID: "task-1"}, "member@example.com")
			return errorValue
		}},
		{name: "delete", invoke: func(service Service, requestContext context.Context) error {
			_, errorValue := service.deleteTask(requestContext, "task-1", "member@example.com")
			return errorValue
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestStarted := make(chan bool, 1)
			service := Service{
				Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
				HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
					if isDirectoryPeopleRequest(request) {
						return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
					}
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

func TestTaskAddPropagatesTypedFields(t *testing.T) {
	var payload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"Member","email":"member@example.com"},{"id":"kim","name":"김인턴","email":"kim@example.com","mattermostUsername":"internkim"}],"definitions":{"categories":["여명거리","김인턴"],"types":["기능","문서"]}}`)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return taskToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	_, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":" 고객지원 분기 결산 누락 항목 확인 ","size":" s ","status":"planned","business":" 김인턴 ","type":"문서","startsAt":" 2026-07-15 ","endsAt":" 2026-07-17 ","participantPersonHints":["@internkim"]}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "member@example.com",
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
	if payload["size"] != "S" || payload["status"] != "planned" || payload["startDate"] != "2026-07-15" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["business"] != "김인턴" || payload["type"] != "문서" {
		t.Fatalf("labels = business %#v type %#v", payload["business"], payload["type"])
	}
	for _, fieldName := range []string{"prompt", "title", "targetPersonHint", "participantPersonHints", "requesterEmail", "source", "weekCode", "allowDuplicate", "duplicatePolicy"} {
		if _, found := payload[fieldName]; found {
			t.Fatalf("unexpected payload field %q in %#v", fieldName, payload)
		}
	}
	participantIDs, ok := payload["participantIDs"].([]any)
	// Naming a colleague makes the task theirs; the person filing it is not added.
	if !ok || len(participantIDs) != 1 || participantIDs[0] != "kim" {
		t.Fatalf("participantIDs = %#v", payload["participantIDs"])
	}
}

func TestTaskAddRefusesUnregisteredBusinessWithoutPosting(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state" {
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"Member","email":"member@example.com"}],"definitions":{"categories":["여명거리","김인턴"],"types":["기능","문서"]}}`)), nil
			}
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"업무","business":"없는사업"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_label_not_registered" {
		t.Fatalf("response = %+v", response)
	}
}

func TestDecodeTaskAddInputRejectsLegacyPromptAndContent(t *testing.T) {
	for _, document := range []string{`{"prompt":"업무 추가"}`, `{"content":"업무 추가"}`} {
		if _, errorValue := decodeTaskAddInput([]byte(document)); errorValue == nil || !strings.Contains(errorValue.Error(), "unknown field") {
			t.Fatalf("error = %v for %s", errorValue, document)
		}
	}
}

func TestDecodeTaskAddInputTrimsCanonicalFields(t *testing.T) {
	input, errorValue := decodeTaskAddInput([]byte(`{"title":" 정확한 제목 ","size":" m ","status":" in_progress ","startsAt":" 2026-07-15 ","endsAt":" 2026-07-17 ","participantPersonHints":[" @kim ","@kim"," "]}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.Title != "정확한 제목" || input.Size != "M" || input.Status != "in_progress" || input.StartsAt != "2026-07-15" || input.EndsAt != "2026-07-17" || len(input.ParticipantPersonHints) != 1 || input.ParticipantPersonHints[0] != "@kim" {
		t.Fatalf("input = %+v", input)
	}
}

func TestDecodeTaskAddInputRejectsInvalidTypedFields(t *testing.T) {
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
		_, errorValue := decodeTaskAddInput([]byte(testCase.document))
		if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.errorMessage) {
			t.Fatalf("error = %v for %s", errorValue, testCase.document)
		}
	}
}

func TestTaskAddReturnsAmbiguousOwnerError(t *testing.T) {
	postCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"lee","name":"샘플","email":"lee@example.com","mattermostUsername":"lee"},{"id":"kim","name":"샘플","email":"kim@example.com","mattermostUsername":"kim"}]}`)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				postCalled = true
				return taskToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"업무 요청","participantPersonHints":["샘플"]}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "member@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "interaction_required" {
		t.Fatalf("response = %+v", response)
	}
	if postCalled {
		t.Fatal("post should not be called for ambiguous owner")
	}
	if !strings.Contains(string(response.Result), "task_owner_ambiguous") {
		t.Fatalf("result should keep the domain code: %s", string(response.Result))
	}
	if !strings.Contains(string(response.Result), "@lee") || !strings.Contains(string(response.Result), "@kim") {
		t.Fatalf("result = %s", string(response.Result))
	}
	if !strings.Contains(string(response.Result), `"toolNames":["ask_input"]`) {
		t.Fatalf("result should steer recovery to ask_input: %s", string(response.Result))
	}
}

func TestTaskAddReturnsParticipantResolutionErrorBeforeCreate(t *testing.T) {
	postCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"Member","email":"member@example.com"},{"id":"lee","name":"샘플","email":"lee@example.com"},{"id":"kim","name":"샘플","email":"kim@example.com"}]}`)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				postCalled = true
				return taskToolJSONResponse(`{"id":"task-1"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"공동 업무","participantPersonHints":["샘플"]}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "interaction_required" || postCalled {
		t.Fatalf("response=%+v postCalled=%t", response, postCalled)
	}
	// The first person named owns the task, so an ambiguous name fails as an owner.
	if !strings.Contains(string(response.Result), "task_owner_ambiguous") {
		t.Fatalf("result should keep the domain code: %s", string(response.Result))
	}
	if !strings.Contains(string(response.Result), `"toolNames":["ask_input"]`) {
		t.Fatalf("result should steer recovery to ask_input: %s", string(response.Result))
	}
}

func TestTaskListFiltersTasksByQueryIgnoringSpaces(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method != http.MethodGet || request.URL.String() != "http://internkim/task/api/state" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"currentWeek":{"code":"26W23"},"members":[{"id":"member","name":"Member","email":"member@example.com"}],"tasks":[{"id":"task-1","ownerID":"member","ownerName":"Member","content":"견본코리아 기획안 전달","status":"planned","weekCode":"26W23"},{"id":"task-2","ownerID":"member","ownerName":"Member","content":"사무실 미팅","status":"planned","weekCode":"26W23"}]}`)), nil
		})},
	}

	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"query":"견본 코리아","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
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

func TestTaskAddAddsParticipantPresentations(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"rain","name":"김테스트","email":"rain@example.com","mattermostUsername":"rain"}]}`)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				return taskToolJSONResponse(`{"id":"task-1","participantIDs":["rain"],"participantNames":["김테스트"],"content":"경산 일정","status":"in_progress"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
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
	if result.TaskID != "task-1" || len(result.ParticipantPresentations) != 1 || result.ParticipantPresentations[0].NotifyMention != "@김테스트" {
		t.Fatalf("participant presentations = %+v", result.ParticipantPresentations)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "task-1" || response.Effects[0].Effect != "created" {
		t.Fatalf("task_add effects = %+v", response)
	}
}

func TestTaskAddReportsDuplicateAsTypedFailure(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet {
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"Member","email":"member@example.com"}]}`)), nil
			}
			return taskToolJSONResponse(`{"status":"skipped_duplicate","duplicateTask":{"id":"task-existing"}}`), nil
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"고객지원 분기 결산"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "task_duplicate" || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestTaskAddDeduplicatesSameTitleSameOwnerWithinWindow(t *testing.T) {
	postCalled := false
	recentCreatedAt := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(fmt.Sprintf(`{"members":[{"id":"member","name":"Member","email":"member@example.com"}],"tasks":[{"id":"task-existing","ownerID":"member","ownerName":"Member","content":"고객지원 분기 결산 누락 항목 확인","status":"planned","createdAt":%q}]}`, recentCreatedAt)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				postCalled = true
				return taskToolJSONResponse(`{"id":"task-new"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"고객지원 분기 결산 누락 항목 확인"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if postCalled {
		t.Fatal("task_add should not create a second record for a recent same-title same-owner duplicate")
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("response = %+v", response)
	}
	var result struct {
		TaskID string `json:"taskID"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.TaskID != "task-existing" {
		t.Fatalf("taskID = %q, expected the first task's ID", result.TaskID)
	}
	if strings.Contains(string(response.Result), "createdAt") {
		t.Fatalf("result leaked internal createdAt field: %s", response.Result)
	}
}

func TestTaskAddCreatesNewTaskForDifferentTitle(t *testing.T) {
	recentCreatedAt := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(fmt.Sprintf(`{"members":[{"id":"member","name":"Member","email":"member@example.com"}],"tasks":[{"id":"task-existing","ownerID":"member","ownerName":"Member","content":"다른 업무","status":"planned","createdAt":%q}]}`, recentCreatedAt)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				return taskToolJSONResponse(`{"id":"task-new"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"고객지원 분기 결산 누락 항목 확인"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		TaskID string `json:"taskID"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.TaskID != "task-new" {
		t.Fatalf("taskID = %q, expected a new task to be created for a different title", result.TaskID)
	}
}

func TestTaskAddCreatesNewTaskForDifferentOwner(t *testing.T) {
	recentCreatedAt := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(fmt.Sprintf(`{"members":[{"id":"member","name":"Member","email":"member@example.com"}],"tasks":[{"id":"task-existing","ownerID":"kim","ownerName":"Kim","content":"고객지원 분기 결산 누락 항목 확인","status":"planned","createdAt":%q}]}`, recentCreatedAt)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				return taskToolJSONResponse(`{"id":"task-new"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"고객지원 분기 결산 누락 항목 확인"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		TaskID string `json:"taskID"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.TaskID != "task-new" {
		t.Fatalf("taskID = %q, expected a new task to be created for a different owner", result.TaskID)
	}
}

func TestTaskAddCreatesNewTaskAfterDuplicateWindowExpires(t *testing.T) {
	staleCreatedAt := time.Now().UTC().Add(-15 * time.Minute).Format(time.RFC3339)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(fmt.Sprintf(`{"members":[{"id":"member","name":"Member","email":"member@example.com"}],"tasks":[{"id":"task-existing","ownerID":"member","ownerName":"Member","content":"고객지원 분기 결산 누락 항목 확인","status":"planned","createdAt":%q}]}`, staleCreatedAt)), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				return taskToolJSONResponse(`{"id":"task-new"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskAdd(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_add",
		Input:    []byte(`{"title":"고객지원 분기 결산 누락 항목 확인"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var result struct {
		TaskID string `json:"taskID"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result.TaskID != "task-new" {
		t.Fatalf("taskID = %q, expected a new task after the duplicate window expired", result.TaskID)
	}
}

func TestTaskUpdateUsesSharedPutAPIWithoutCreatingTask(t *testing.T) {
	postCalled := false
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"foreign","name":"Foreign","email":"foreign@example.com"},{"id":"member","name":"Member","email":"member@example.com"}],"tasks":[{"id":"foreign-task","ownerID":"foreign","ownerName":"Foreign","participantIDs":["foreign"],"content":"10분 회의","status":"in_progress","weekCode":"26W24"},{"id":"task-1","ownerID":"member","ownerName":"Member","participantIDs":["member"],"participantNames":["Member"],"business":"개발","type":"회의","content":"10분 회의","size":"XS","status":"in_progress","weekCode":"26W24"}]}`)), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://internkim/task/api/tasks/task-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return taskToolJSONResponse(`{"id":"task-1","content":"15분 회의","status":"in_progress"}`), nil
			case request.Method == http.MethodPost && request.URL.String() == "http://internkim/task/api/tasks":
				postCalled = true
				return taskToolJSONResponse(`{"id":"new-task"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"task-1","title":"15분 회의"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "member@example.com",
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || response.Status != "in_progress" {
		t.Fatalf("response = %+v", response)
	}
	if postCalled {
		t.Fatal("update must not call quick create")
	}
	if updatedPayload["content"] != "15분 회의" || updatedPayload["status"] != "in_progress" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || len(response.Effects) != 1 || response.Effects[0].ID != "task-1" || response.Effects[0].Effect != "updated" {
		t.Fatalf("task_update effects = %+v", response)
	}
}

func TestTaskUpdateRejectsMismatchedBackendTaskID(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet {
				return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"이전 업무"}]}`), nil
			}
			return taskToolJSONResponse(`{"id":"task-2","content":"수정 업무"}`), nil
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"task-1","title":"수정 업무"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "task_result_invalid" || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestTaskUpdateRejectsBackendNoOp(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet {
				return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"이전 업무"}]}`), nil
			}
			return taskToolJSONResponse(`{"id":"task-1","content":"이전 업무"}`), nil
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"task-1","title":"수정 업무"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !response.IsError || response.ErrorCode != "task_result_invalid" || len(response.Effects) != 0 {
		t.Fatalf("response = %+v", response)
	}
}

func TestDecodeTaskUpdateInputRejectsLegacyAndResolutionFields(t *testing.T) {
	for _, document := range []string{
		`{"taskHint":"task-1","content":"이전 제목"}`,
		`{"taskHint":"task-1","query":"이전 제목","status":"completed"}`,
		`{"taskHint":"task-1","targetPersonHint":"member","status":"completed"}`,
		`{"taskHint":"task-1","weekCode":"26W24","status":"completed"}`,
	} {
		_, errorValue := decodeTaskUpdateInput([]byte(document))
		if errorValue == nil || !strings.Contains(errorValue.Error(), "unknown field") {
			t.Fatalf("document = %s error = %v", document, errorValue)
		}
	}
}

func TestDecodeTaskUpdateInputRequiresTaskHintAndPatch(t *testing.T) {
	for _, document := range []string{
		`{"status":"completed"}`,
		`{"taskHint":"task-1"}`,
		`{"taskHint":" "}`,
	} {
		if _, errorValue := decodeTaskUpdateInput([]byte(document)); errorValue == nil {
			t.Fatalf("document = %s expected validation error", document)
		}
	}
}

func TestDecodeTaskUpdateInputTrimsTitle(t *testing.T) {
	input, errorValue := decodeTaskUpdateInput([]byte(`{"taskHint":" task-1 ","title":" 정확한 제목 "}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if input.TaskHint != "task-1" || input.Title == nil || *input.Title != "정확한 제목" {
		t.Fatalf("input = %+v", input)
	}
}

func TestTaskUpdateResolvesByExactTaskIDAcrossAllTasks(t *testing.T) {
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"deck-1","ownerID":"member","ownerName":"이샘플","participantIDs":["member"],"business":"샘플거리","type":"문서","content":"IR 덱","status":"in_progress","weekCode":"26W28"}]}`)), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://internkim/task/api/tasks/deck-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return taskToolJSONResponse(`{"id":"deck-1","status":"completed"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"deck-1","status":"completed"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected exact taskID hint to resolve, got error response %+v", response)
	}
	if updatedPayload["status"] != "completed" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestTaskUpdateResolvesByExactUniqueTitle(t *testing.T) {
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"member","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"deck-1","ownerID":"member","ownerName":"이샘플","participantIDs":["member"],"content":"IR 덱","status":"in_progress","weekCode":"26W28"}]}`)), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://internkim/task/api/tasks/deck-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return taskToolJSONResponse(`{"id":"deck-1","status":"completed"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"IR 덱","status":"completed"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected exact unique title hint to resolve, got error response %+v", response)
	}
	if updatedPayload["status"] != "completed" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestTaskUpdateAmbiguousTitleReturnsCandidates(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[],"tasks":[{"id":"task-1","content":"IR 덱","status":"in_progress"},{"id":"task-2","content":"IR 덱","status":"planned"}]}`)), nil
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"IR 덱","status":"completed"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(string(response.Result), "task-1") || !strings.Contains(string(response.Result), "task-2") {
		t.Fatalf("expected both ambiguous candidates, got result = %s", response.Result)
	}
}

func TestTaskUpdateAHintNothingComesCloseToWritesNothingAndNamesNothing(t *testing.T) {
	putCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodPut {
				putCalled = true
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[],"tasks":[{"id":"task-1","content":"IR Deck","status":"in_progress"}]}`)), nil
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"zzzz","status":"completed"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if strings.Contains(string(response.Result), "task-1") {
		t.Fatalf("a task nothing was asked about is not a candidate, got result = %s", response.Result)
	}
	if putCalled {
		t.Fatal("unresolved taskHint must not write")
	}
}

func TestTaskUpdateResolvesATitleWhoseCaseDiffers(t *testing.T) {
	putCalled := false
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodPut {
				putCalled = true
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[],"tasks":[{"id":"task-1","content":"IR Deck","status":"in_progress"}]}`)), nil
		})},
	}

	if _, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"ir deck","status":"completed"}`),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !putCalled {
		t.Fatal("a title is the same title in another case, so the update had to reach the write")
	}
}

func TestTaskUpdateHintResolutionTrimsWhitespaceBeforeMatching(t *testing.T) {
	var updatedPayload map[string]any
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[],"tasks":[{"id":"task-1","content":"IR 덱","status":"in_progress"}]}`)), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://internkim/task/api/tasks/task-1":
				if errorValue := json.NewDecoder(request.Body).Decode(&updatedPayload); errorValue != nil {
					t.Fatal(errorValue)
				}
				return taskToolJSONResponse(`{"id":"task-1","status":"completed"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":" IR 덱 ","status":"completed"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("expected trimmed title hint to resolve, got error response %+v", response)
	}
	if updatedPayload["status"] != "completed" {
		t.Fatalf("updated payload = %+v", updatedPayload)
	}
}

func TestTaskDeleteUsesSharedDeleteAPI(t *testing.T) {
	var deletedRequesterEmail string
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"고객지원 분기 결산 검토 완료"}]}`), nil
			case request.Method == http.MethodDelete && request.URL.String() == "http://internkim/task/api/tasks/task-1":
				deletedRequesterEmail = request.Header.Get(admindRequesterEmailHeader)
				return taskToolJSONResponse(`{"status":"deleted","task":{"id":"task-1"}}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_delete",
		Input:    []byte(`{"taskHint":"task-1"}`),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail: "member@example.com",
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
		t.Fatalf("task_delete effects = %+v", response)
	}
	if deletedRequesterEmail != "member@example.com" {
		t.Fatalf("requester email = %q", deletedRequesterEmail)
	}
}

func TestTaskDeleteResolvesByExactUniqueTitle(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"고객지원 분기 결산 검토 완료"}]}`), nil
			case request.Method == http.MethodDelete && request.URL.String() == "http://internkim/task/api/tasks/task-1":
				return taskToolJSONResponse(`{"status":"deleted","task":{"id":"task-1"}}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}

	response, errorValue := service.invokeTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_delete",
		Input:    []byte(`{"taskHint":"고객지원 분기 결산 검토 완료"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Status != "deleted" {
		t.Fatalf("expected exact unique title hint to resolve and delete, got response = %+v", response)
	}
}

func TestTaskDeleteAmbiguousTitleReturnsCandidatesWithoutDeleting(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"IR 덱"},{"id":"task-2","content":"IR 덱"}]}`), nil
		})},
	}

	response, errorValue := service.invokeTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_delete",
		Input:    []byte(`{"taskHint":"IR 덱"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(string(response.Result), "task-1") || !strings.Contains(string(response.Result), "task-2") {
		t.Fatalf("expected both ambiguous candidates, got result = %s", response.Result)
	}
}

func TestTaskDeleteAHintNothingComesCloseToDeletesNothingAndNamesNothing(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method != http.MethodGet {
				t.Fatalf("unexpected write request %s %s", request.Method, request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[],"tasks":[{"id":"task-1","content":"IR Deck","status":"in_progress"}]}`)), nil
		})},
	}

	response, errorValue := service.invokeTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_delete",
		Input:    []byte(`{"taskHint":"zzzz"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_hint_unresolved" || !response.SafeRetry {
		t.Fatalf("response = %+v", response)
	}
	if strings.Contains(string(response.Result), "task-1") {
		t.Fatalf("a task nothing was asked about is not a candidate, got result = %s", response.Result)
	}
}

func TestDecodeTaskDeleteInputRejectsResolutionFields(t *testing.T) {
	for _, document := range []string{
		`{"query":"회의"}`,
		`{"taskHint":"task-1","targetPersonHint":"member"}`,
		`{"taskHint":"task-1","weekCode":"26W24"}`,
	} {
		_, errorValue := decodeTaskDeleteInput([]byte(document))
		if errorValue == nil {
			t.Fatalf("document = %s expected validation error", document)
		}
	}
}

func TestTaskDeleteNotFoundReturnsTypedFailure(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet {
				return taskToolJSONResponse(`{"tasks":[{"id":"missing-task","content":"회의"}]}`), nil
			}
			response := taskToolJSONResponse(`{"error":"task not found"}`)
			response.StatusCode = http.StatusNotFound
			return response, nil
		})},
	}

	response, errorValue := service.invokeTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_delete",
		Input:    []byte(`{"taskHint":"missing-task"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_not_found" || response.Status == "deleted" {
		t.Fatalf("response = %+v", response)
	}
}

func TestTaskDeleteReturnsTypedNotFoundWithoutExactEvidence(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet {
				return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"회의"}]}`), nil
			}
			return taskToolJSONResponse(`{"status":"deleted","task":{"id":"other-task"}}`), nil
		})},
	}

	response, errorValue := service.invokeTaskDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_delete",
		Input:    []byte(`{"taskHint":"task-1"}`),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_not_found" || response.Status == "deleted" {
		t.Fatalf("response = %+v error = %v", response, errorValue)
	}
}

func taskListTwoOwnerStateService(t *testing.T) Service {
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.URL.String() != "http://internkim/task/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"currentWeek":{"code":"26W25"},"members":[{"id":"rain","name":"김테스트","email":"rain@example.com"},{"id":"lee","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"rain-future","ownerID":"rain","ownerName":"김테스트","content":"김테스트 예정 업무","status":"planned","weekCode":"26W30"},{"id":"rain-done","ownerID":"rain","ownerName":"김테스트","content":"김테스트 완료 업무","status":"completed","weekCode":"26W25"},{"id":"lee-task","ownerID":"lee","ownerName":"이샘플","content":"이샘플 업무","status":"planned","weekCode":"26W25"}]}`)), nil
		})},
	}
}

func TestTaskListEmptyHintDefaultsToRequester(t *testing.T) {
	service := taskListTwoOwnerStateService(t)
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
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

func TestTaskListAllScopeListsEveryone(t *testing.T) {
	service := taskListTwoOwnerStateService(t)
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
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

func TestDecodeTaskListInputRejectsTaskAddFields(t *testing.T) {
	_, errorValue := decodeTaskListInput([]byte(`{"prompt":"업무 추가","title":"분기 결산","endsAt":"2026-07-17"}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "unknown field") {
		t.Fatalf("expected task_add fields to fail task_list validation, got %v", errorValue)
	}
}

func TestTaskListOwnNameNarrowsToRequester(t *testing.T) {
	service := taskListTwoOwnerStateService(t)
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"participantPersonHint":"김테스트","weekFrom":-1000}`),
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

func TestTaskListFiltersByCanonicalStatusAcrossEveryone(t *testing.T) {
	service := taskListTwoOwnerStateService(t)
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"scope":"all","status":"planned","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	result := string(response.Result)
	if !strings.Contains(result, "rain-future") || !strings.Contains(result, "lee-task") || strings.Contains(result, "rain-done") {
		t.Fatalf("expected everyone's planned tasks across weeks, got %s", result)
	}
	if !strings.Contains(result, `"statusFilter":"planned"`) {
		t.Fatalf("expected the canonical filter echoed back, got %s", result)
	}
}

func TestTaskListTargetPersonHintReturnsThatPerson(t *testing.T) {
	service := taskListTwoOwnerStateService(t)
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"participantPersonHint":"이샘플","weekFrom":-1000}`),
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

func TestTaskListUnknownPersonDoesNotListEveryone(t *testing.T) {
	service := taskListTwoOwnerStateService(t)
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"participantPersonHint":"Expensive","weekFrom":-1000}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "rain@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_owner_not_found" {
		t.Fatalf("expected unresolved person error, got %+v", response)
	}
	if strings.Contains(string(response.Result), "rain-future") || strings.Contains(string(response.Result), "lee-task") {
		t.Fatalf("expected no task disclosure, got %s", response.Result)
	}
}

func TestTaskListTargetPersonHintIncludesParticipantTasks(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.URL.String() != "http://internkim/task/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"currentWeek":{"code":"26W25"},"members":[{"id":"owner","name":"오너","email":"owner@example.com"},{"id":"lee","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"owner-task","ownerID":"owner","ownerName":"오너","participantIDs":["owner","lee"],"participantNames":["오너","이샘플"],"content":"이샘플 참여 업무","status":"in_progress","weekCode":"26W25"},{"id":"other-task","ownerID":"owner","ownerName":"오너","participantIDs":["owner"],"participantNames":["오너"],"content":"오너 단독 업무","status":"in_progress","weekCode":"26W25"}]}`)), nil
		})},
	}
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"participantPersonHint":"이샘플","weekFrom":-1000}`),
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

func TestTaskListTreatsAvailablePlannedAndPausedAsCurrentWeek(t *testing.T) {
	now := time.Now()
	thisWeek := weekCodeForTaskDate(now)
	priorWeek := weekCodeForTaskDate(now.AddDate(0, 0, -14))
	futureStartDate := now.AddDate(0, 0, 14).Format("2006-01-02")
	stateBody := fmt.Sprintf(`{"currentWeek":{"code":%q},"members":[{"id":"lee","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"planned-old-week","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"예정 업무","status":"planned","weekCode":%q},{"id":"planned-future-start","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"미래 예정 업무","status":"planned","startDate":%q,"weekCode":%q},{"id":"paused-old-week","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"일시정지 업무","status":"paused","weekCode":%q}]}`, thisWeek, priorWeek, futureStartDate, priorWeek, priorWeek)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.URL.String() != "http://internkim/task/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, stateBody)), nil
		})},
	}
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"participantPersonHint":"이샘플"}`),
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

func TestTaskListClassifiesFinishedInactiveTasksByDates(t *testing.T) {
	now := time.Now()
	thisWeek := weekCodeForTaskDate(now)
	oldWeek := weekCodeForTaskDate(now.AddDate(0, 0, -28))
	thisWeekDate := now.Format("2006-01-02")
	priorWeekDate := now.AddDate(0, 0, -7).Format("2006-01-02")
	stateBody := fmt.Sprintf(`{"currentWeek":{"code":%q},"members":[{"id":"lee","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"completed-current-end","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"완료 업무","status":"completed","startDate":%q,"endDate":%q,"weekCode":%q},{"id":"rejected-current-end","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"기각 업무","status":"rejected","startDate":%q,"endDate":%q,"weekCode":%q},{"id":"stopped-current-start","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"중단 시작일 업무","status":"stopped","startDate":%q,"weekCode":%q},{"id":"stopped-prior-end","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"participantNames":["이샘플"],"content":"중단 종료일 우선 업무","status":"stopped","startDate":%q,"endDate":%q,"weekCode":%q}]}`, thisWeek, priorWeekDate, thisWeekDate, oldWeek, priorWeekDate, thisWeekDate, oldWeek, thisWeekDate, oldWeek, thisWeekDate, priorWeekDate, oldWeek)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.URL.String() != "http://internkim/task/api/state" {
				t.Fatalf("expected all-tasks state endpoint, got %s", request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, stateBody)), nil
		})},
	}
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
		Input:    []byte(`{"participantPersonHint":"이샘플"}`),
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

func TestTaskListWeekCodes(t *testing.T) {
	now := time.Date(2026, time.June, 18, 12, 0, 0, 0, time.UTC)
	thisWeek := weekCodeForTaskDate(now)
	single, errorValue := taskListWeekCodes(0, 0, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(single) != 1 || !single[thisWeek] {
		t.Fatalf("expected only this week %q, got %v", thisWeek, single)
	}
	threeWeeks, errorValue := taskListWeekCodes(-2, 0, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(threeWeeks) != 3 || !threeWeeks[thisWeek] ||
		!threeWeeks[weekCodeForTaskDate(now.AddDate(0, 0, -7))] ||
		!threeWeeks[weekCodeForTaskDate(now.AddDate(0, 0, -14))] {
		t.Fatalf("expected this and the prior two weeks, got %v", threeWeeks)
	}
	swappedWeeks, errorValue := taskListWeekCodes(0, -1, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(swappedWeeks) != 2 {
		t.Fatalf("expected swapped bounds to span 2 weeks")
	}
	wideWeeks, errorValue := taskListWeekCodes(-1000, 0, thisWeek)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if wideWeeks != nil {
		t.Fatalf("expected no week filter for a very wide range")
	}
	if _, errorValue := taskListWeekCodes(0, 0, "invalid"); errorValue == nil {
		t.Fatal("expected an invalid canonical week to fail closed")
	}
}

func TestTaskListDefaultsToThisWeekOnly(t *testing.T) {
	thisWeek := "26W30"
	priorWeek := "26W29"
	stateBody := fmt.Sprintf(`{"currentWeek":{"code":%q},"members":[{"id":"lee","name":"이샘플","email":"lee@example.com"}],"tasks":[{"id":"this-week-task","ownerID":"lee","ownerName":"이샘플","content":"이번주 업무","status":"in_progress","weekCode":%q},{"id":"prior-week-task","ownerID":"lee","ownerName":"이샘플","content":"지난 업무","status":"in_progress","weekCode":%q}]}`, thisWeek, thisWeek, priorWeek)
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, stateBody)), nil
		})},
	}
	response, errorValue := service.invokeTaskList(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_list",
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

func taskToolJSONResponse(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(document)),
	}
}

func TestTaskAddTakesTheStatusesTheSchemaPromises(t *testing.T) {
	for _, promised := range []struct{ english, stored string }{
		{"planned", "planned"},
		{"in_progress", "in_progress"},
		{"completed", "completed"},
		{"paused", "paused"},
		{"rejected", "rejected"},
		{"stopped", "stopped"},
	} {
		document := json.RawMessage(`{"title":"운동","status":"` + promised.english + `"}`)
		input, errorValue := decodeTaskAddInput(document)
		if errorValue != nil {
			t.Fatalf("the schema offers %q and the tool refused it: %v", promised.english, errorValue)
		}
		if input.Status != promised.stored {
			t.Fatalf("%q became %q, expected %q", promised.english, input.Status, promised.stored)
		}
	}
}

func TestTaskUpdateTakesTheStatusesTheSchemaPromises(t *testing.T) {
	document := json.RawMessage(`{"taskHint":"운동","status":"completed"}`)

	input, errorValue := decodeTaskUpdateInput(document)

	if errorValue != nil {
		t.Fatalf("task_update refused a status its schema offers: %v", errorValue)
	}
	if input.Status == nil || *input.Status != "completed" {
		t.Fatalf("status = %v", input.Status)
	}
}

func TestTaskAddStillRefusesAStatusNobodyOffers(t *testing.T) {
	if _, errorValue := decodeTaskAddInput(json.RawMessage(`{"title":"운동","status":"nonsense"}`)); errorValue == nil {
		t.Fatal("a status outside the vocabulary must still be refused")
	}
}

func TestPersonListReturnsTheRosterHintsResolveAgainst(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method != http.MethodGet || request.URL.String() != "http://internkim/task/api/state" {
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			}
			return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"person-sample","name":"이샘플","email":"sample@example.com","mattermostUsername":"sampleuser"}]}`)), nil
		})},
	}

	response, errorValue := service.invokeTaskTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "person_list",
		Input:    []byte(`{}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "sample@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("response = %+v", response)
	}
	for _, expected := range []string{`"count":1`, `"personID":"person-sample"`, `"name":"이샘플"`, `"mention":"@이샘플"`} {
		if !strings.Contains(string(response.Result), expected) {
			t.Fatalf("result missing %s: %s", expected, string(response.Result))
		}
	}
}

func taskParticipantService(t *testing.T, capturedPayload *string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			switch {
			case request.Method == http.MethodGet && request.URL.String() == "http://internkim/task/api/state":
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"lee","name":"이샘플","email":"lee@example.com"},{"id":"shin","name":"신견본","email":"shin@example.com"}],"tasks":[{"id":"task-1","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee"],"content":"운동","status":"completed"}]}`)), nil
			case request.Method == http.MethodPut && request.URL.String() == "http://internkim/task/api/tasks/task-1":
				body, _ := io.ReadAll(request.Body)
				*capturedPayload = string(body)
				var payload struct {
					OwnerID        string   `json:"ownerID"`
					ParticipantIDs []string `json:"participantIDs"`
					Content        string   `json:"content"`
				}
				if json.Unmarshal(body, &payload) != nil {
					t.Fatalf("payload = %s", body)
				}
				participants, _ := json.Marshal(payload.ParticipantIDs)
				return taskToolJSONResponse(`{"id":"task-1","ownerID":"` + payload.OwnerID + `","participantIDs":` + string(participants) + `,"content":"` + payload.Content + `","status":"completed"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
				return nil, nil
			}
		})},
	}
}

func TestTaskUpdateReplacesParticipantsFromPersonHints(t *testing.T) {
	capturedPayload := ""
	service := taskParticipantService(t, &capturedPayload)

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"운동","participantPersonHints":["견본"]}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(capturedPayload, `"participantIDs":["lee","shin"]`) {
		t.Fatalf("payload = %s", capturedPayload)
	}
}

func TestTaskUpdateRefusesAnUnknownParticipantBeforeWriting(t *testing.T) {
	capturedPayload := ""
	service := taskParticipantService(t, &capturedPayload)

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"운동","participantPersonHints":["없는사람"]}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "lee@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || capturedPayload != "" {
		t.Fatalf("response=%+v payload=%s", response, capturedPayload)
	}
	for _, expected := range []string{"task_participant_not_found", `"toolNames":["person_list","ask_input"]`, `"name":"신견본"`} {
		if !strings.Contains(string(response.Result), expected) {
			t.Fatalf("result missing %s: %s", expected, string(response.Result))
		}
	}
}

func TestTaskUpdateExplainsWhoMayChangeParticipants(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if isDirectoryPeopleRequest(request) {
				return directoryPeopleTestResponse(directoryPeopleTestDocument), nil
			}
			if request.Method == http.MethodGet {
				return taskToolJSONResponse(useDirectoryPeopleOfTaskStateAnd(t, `{"members":[{"id":"lee","name":"이샘플","email":"lee@example.com"},{"id":"shin","name":"신견본","email":"shin@example.com"}],"tasks":[{"id":"task-1","ownerID":"lee","ownerName":"이샘플","participantIDs":["lee","shin"],"content":"운동","status":"completed"}]}`)), nil
			}
			return &http.Response{StatusCode: http.StatusForbidden, Body: io.NopCloser(strings.NewReader("flow access required")), Header: http.Header{}}, nil
		})},
	}

	response, errorValue := service.invokeTaskUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "task_update",
		Input:    []byte(`{"taskHint":"운동","participantPersonHints":["신견본"]}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "shin@example.com"},
	})
	if errorValue != nil {
		t.Fatalf("a refusal must be a typed tool failure, not a transport error: %v", errorValue)
	}
	if !response.IsError || response.ErrorCode != "task_assignment_forbidden" {
		t.Fatalf("response = %+v", response)
	}
	if !strings.Contains(response.Message, "이샘플") {
		t.Fatalf("message must name who can make the change: %q", response.Message)
	}
}
