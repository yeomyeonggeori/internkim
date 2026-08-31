package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

type approvalTargetFixture struct {
	toolName      string
	inputField    string
	titleHint     string
	absentHint    string
	expectedID    string
	expectedTitle string
	newService    func(*testing.T, *[]string) Service
	invoke        func(Service, context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error)
}

func calendarApprovalTargetService(t *testing.T, methods *[]string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			*methods = append(*methods, request.Method)
			return calendarToolEventsResponse(
				calendarToolEventDocument("event-1", "상하이 acme 미팅", "2026-08-18T14:00:00+09:00", "2026-08-18T15:00:00+09:00"),
				calendarToolEventDocument("event-2", "주간 팀 회의", "2026-08-19T10:00:00+09:00", "2026-08-19T11:00:00+09:00"),
			), nil
		})},
	}
}

func taskApprovalTargetService(t *testing.T, methods *[]string) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			*methods = append(*methods, request.Method)
			return taskToolJSONResponse(`{"tasks":[{"id":"task-1","content":"고객지원 분기 결산 검토 완료"},{"id":"task-2","content":"주간 보고"}]}`), nil
		})},
	}
}

var approvalTargetFixtures = []approvalTargetFixture{
	{
		toolName:      "event_delete",
		inputField:    "eventHint",
		titleHint:     "상하이 acme 미팅",
		absentHint:    "NVIDIA·젯슨 공급 미팅",
		expectedID:    "event-1",
		expectedTitle: "상하이 acme 미팅",
		newService:    calendarApprovalTargetService,
		invoke:        Service.invokeCalendarEventDelete,
	},
	{
		toolName:      "task_delete",
		inputField:    "taskHint",
		titleHint:     "고객지원 분기 결산 검토 완료",
		absentHint:    "NVIDIA·젯슨 공급 미팅",
		expectedID:    "task-1",
		expectedTitle: "고객지원 분기 결산 검토 완료",
		newService:    taskApprovalTargetService,
		invoke:        Service.invokeTaskDelete,
	},
}

func approvalTargetToolInput(inputField string, hint string) json.RawMessage {
	document, _ := json.Marshal(map[string]string{inputField: hint})
	return document
}

func approvalTargetInvokeRequest(toolName string, input json.RawMessage) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: toolName,
		Input:    input,
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "member@example.com"},
	}
}

func resolveApprovalTargetThroughRoute(t *testing.T, service Service, toolName string, input json.RawMessage) capabilities.ToolInvokeResponse {
	t.Helper()
	requestDocument, _ := json.Marshal(approvalTargetInvokeRequest(toolName, input))
	response, errorValue := service.resolveCapabilityToolTarget(context.Background(), toolName, bytes.NewReader(requestDocument))
	if errorValue != nil {
		t.Fatalf("expected the target resolution route to answer: %v", errorValue)
	}
	return response
}

func decodeResolvedApprovalTarget(t *testing.T, response capabilities.ToolInvokeResponse) capabilityToolTarget {
	t.Helper()
	target := capabilityToolTarget{}
	if errorValue := json.Unmarshal(response.Result, &target); errorValue != nil {
		t.Fatalf("expected a resolved target document, got %s: %v", response.Result, errorValue)
	}
	return target
}

func TestResolvingATargetNamesTheEntityAnExactTitleResolvesTo(t *testing.T) {
	for _, fixture := range approvalTargetFixtures {
		t.Run(fixture.toolName, func(t *testing.T) {
			methods := []string{}
			service := fixture.newService(t, &methods)

			response := resolveApprovalTargetThroughRoute(t, service, fixture.toolName, approvalTargetToolInput(fixture.inputField, fixture.titleHint))

			if response.IsError {
				t.Fatalf("expected the hint to resolve, got %+v", response)
			}
			target := decodeResolvedApprovalTarget(t, response)
			if target.ID != fixture.expectedID || target.Title != fixture.expectedTitle || target.InputField != fixture.inputField {
				t.Fatalf("the question is worded from this, got %+v", target)
			}
		})
	}
}

func TestEveryApprovalTargetFieldAcceptsTheIdentityItResolvesTo(t *testing.T) {
	for _, fixture := range approvalTargetFixtures {
		t.Run(fixture.toolName, func(t *testing.T) {
			methods := []string{}
			service := fixture.newService(t, &methods)
			byTitle := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, fixture.toolName, approvalTargetToolInput(fixture.inputField, fixture.titleHint)))

			byIdentity := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, service, fixture.toolName, approvalTargetToolInput(byTitle.InputField, byTitle.ID)))

			if byIdentity != byTitle {
				t.Fatalf("narrowing the approved call to the identity is only safe while the field accepts it, got %+v then %+v", byTitle, byIdentity)
			}
		})
	}
}

func TestResolvingATargetReturnsTheSameFailureTheInvokePathWouldHaveReturned(t *testing.T) {
	for _, fixture := range approvalTargetFixtures {
		t.Run(fixture.toolName, func(t *testing.T) {
			resolveMethods := []string{}
			invokeMethods := []string{}
			input := approvalTargetToolInput(fixture.inputField, fixture.absentHint)

			resolveResponse := resolveApprovalTargetThroughRoute(t, fixture.newService(t, &resolveMethods), fixture.toolName, input)
			invokeResponse, errorValue := fixture.invoke(fixture.newService(t, &invokeMethods), context.Background(), approvalTargetInvokeRequest(fixture.toolName, input))
			if errorValue != nil {
				t.Fatalf("expected the invoke path to answer: %v", errorValue)
			}

			resolveBody, _ := json.Marshal(resolveResponse)
			invokeBody, _ := json.Marshal(invokeResponse)
			if string(resolveBody) != string(invokeBody) {
				t.Fatalf("one unresolved hint has one answer, got\n%s\n%s", resolveBody, invokeBody)
			}
		})
	}
}

func TestResolvingATargetWritesNothing(t *testing.T) {
	for _, fixture := range approvalTargetFixtures {
		t.Run(fixture.toolName, func(t *testing.T) {
			methods := []string{}
			service := fixture.newService(t, &methods)

			resolveApprovalTargetThroughRoute(t, service, fixture.toolName, approvalTargetToolInput(fixture.inputField, fixture.titleHint))

			for _, method := range methods {
				if method != http.MethodGet {
					t.Fatalf("resolving a target before the question is asked can only read, got %+v", methods)
				}
			}
			if len(methods) == 0 {
				t.Fatal("expected the resolution to read the current state rather than trust the hint")
			}
		})
	}
}

func TestAToolThatResolvesNoTargetAheadIsReportedAsHavingNone(t *testing.T) {
	methods := []string{}
	service := calendarApprovalTargetService(t, &methods)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", json.RawMessage(`{"targetType":"currentThread","message":"안녕하세요"}`))

	if response.IsError {
		t.Fatalf("a tool with nothing to resolve ahead is not a failure, got %+v", response)
	}
	if decodeResolvedApprovalTarget(t, response).ID != "" {
		t.Fatalf("expected no target, got %s", response.Result)
	}
	if len(methods) != 0 {
		t.Fatalf("a tool with no target resolver reaches no backend, got %+v", methods)
	}
}

func TestEveryToolThatResolvesATargetAheadRequiresApproval(t *testing.T) {
	for _, route := range capabilityToolTargetRoutes {
		descriptor, hasDescriptor := capabilityToolDescriptorFor(route.ToolName)
		if !hasDescriptor {
			t.Fatalf("%s resolves a target ahead of a question nobody asks for it", route.ToolName)
		}
		if !descriptor.RequiresApproval {
			t.Fatalf("resolving %s ahead of time only pays for itself when a human is about to be asked", route.ToolName)
		}
	}
}

func TestEveryToolThatResolvesATargetAheadNamesARequiredFieldOfItsOwnInputSchema(t *testing.T) {
	for _, fixture := range approvalTargetFixtures {
		t.Run(fixture.toolName, func(t *testing.T) {
			methods := []string{}
			target := decodeResolvedApprovalTarget(t, resolveApprovalTargetThroughRoute(t, fixture.newService(t, &methods), fixture.toolName, approvalTargetToolInput(fixture.inputField, fixture.titleHint)))
			descriptor, _ := capabilityToolDescriptorFor(fixture.toolName)

			narrowedInput := approvalTargetToolInput(target.InputField, target.ID)
			if errorValue := capabilityschema.Validate(descriptor.InputSchema, narrowedInput); errorValue != nil {
				t.Fatalf("the narrowed call has to be a call the tool accepts: %v", errorValue)
			}
		})
	}
}

// A preview route answers words for the approval question instead of a
// resolved identity, so the hint round-trip fixtures cannot exercise it; each
// one is proven by its own resolver tests (message_delete:
// TestMessageDeleteApprovalPreviewQuotesTheTargets and the route-path test
// below).
var previewApprovalTargetRouteNames = []string{"message_delete"}

func TestEveryTargetRouteIsCoveredByAFixture(t *testing.T) {
	if len(capabilityToolTargetRoutes) != len(approvalTargetFixtures)+len(previewApprovalTargetRouteNames) {
		t.Fatalf("every route that resolves a target ahead needs the round trip proven, got %d routes and %d fixtures", len(capabilityToolTargetRoutes), len(approvalTargetFixtures)+len(previewApprovalTargetRouteNames))
	}
	for _, route := range capabilityToolTargetRoutes {
		isCovered := false
		for _, fixture := range approvalTargetFixtures {
			isCovered = isCovered || fixture.toolName == route.ToolName
		}
		for _, previewRouteName := range previewApprovalTargetRouteNames {
			isCovered = isCovered || previewRouteName == route.ToolName
		}
		if !isCovered {
			t.Fatalf("%s resolves a target no test resolves back", route.ToolName)
		}
	}
}

func TestTheMessageDeleteTargetRouteAnswersOnItsOwnPath(t *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		json.NewEncoder(writer).Encode(chatdMessageSearchResponse{Candidates: []chatdMessageSearchCandidate{
			{MessageID: "m1", Text: "중복으로 올라간 공지", AuthoredByAssistant: true},
		}})
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}}
	requestBody := `{"input":{"messageIDs":["m1"]},"context":{"requesterEmail":"member@example.com","platform":"buzz"}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/message_delete/target.resolve", strings.NewReader(requestBody))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected the route to answer, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "중복으로 올라간 공지") {
		t.Fatalf("expected the target's own words, got %s", responseRecorder.Body.String())
	}
}

func TestTheTargetResolutionRouteAnswersOnItsOwnPath(t *testing.T) {
	methods := []string{}
	service := calendarApprovalTargetService(t, &methods)
	requestBody := `{"input":{"eventHint":"상하이 acme 미팅"},"context":{"requesterEmail":"member@example.com"}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/event_delete/target.resolve", strings.NewReader(requestBody))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected the route to answer, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), "event-1") {
		t.Fatalf("expected the resolved event, got %s", responseRecorder.Body.String())
	}
}
