package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// The record answers both verbs, so the fake stands in for the plane rather
// than for the device API these tools used to read.
func recordApprovalTargetService(inputField string, title string, identity string) func(*testing.T, *[]string) Service {
	return func(t *testing.T, paths *[]string) Service {
		t.Helper()
		return Service{
			Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
			HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				*paths = append(*paths, request.URL.Path)
				body, _ := io.ReadAll(request.Body)
				if !strings.Contains(string(body), title) && !strings.Contains(string(body), identity) {
					return recordToolJSONResponse(http.StatusConflict, `{"error":"no event matches that","errorCode":"interaction_required","failureStage":"target_resolution","retryable":true,"safeRetry":true,"candidates":[]}`), nil
				}
				return recordToolJSONResponse(http.StatusOK, `{"tool":"x","target":{"inputField":"`+inputField+`","id":"`+identity+`","title":"`+title+`"}}`), nil
			})},
		}
	}
}

func recordToolJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}
}

var approvalTargetFixtures = []approvalTargetFixture{
	{
		toolName:      "event_delete",
		inputField:    "eventHint",
		titleHint:     "부산 공급사 미팅",
		absentHint:    "서울 거래처 미팅",
		expectedID:    "event-1",
		expectedTitle: "부산 공급사 미팅",
		newService:    recordApprovalTargetService("eventHint", "부산 공급사 미팅", "event-1"),
		invoke:        Service.previewRecordToolTarget,
	},
	{
		toolName:      "task_delete",
		inputField:    "taskHint",
		titleHint:     "고객지원 분기 결산 검토 완료",
		absentHint:    "서울 거래처 미팅",
		expectedID:    "task-1",
		expectedTitle: "고객지원 분기 결산 검토 완료",
		newService:    recordApprovalTargetService("taskHint", "고객지원 분기 결산 검토 완료", "task-1"),
		invoke:        Service.previewRecordToolTarget,
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

func decodeResolvedApprovalTarget(t *testing.T, response capabilities.ToolInvokeResponse) capabilities.ApprovalTarget {
	t.Helper()
	target := capabilities.ApprovalTarget{}
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
			paths := []string{}
			service := fixture.newService(t, &paths)

			resolveApprovalTargetThroughRoute(t, service, fixture.toolName, approvalTargetToolInput(fixture.inputField, fixture.titleHint))

			for _, path := range paths {
				if !strings.HasSuffix(path, "/target") {
					t.Fatalf("resolving a target before the question is asked can only ask what it would touch, got %+v", paths)
				}
			}
			if len(paths) == 0 {
				t.Fatal("expected the resolution to read the current state rather than trust the hint")
			}
		})
	}
}

func TestAToolThatResolvesNoTargetAheadIsReportedAsHavingNone(t *testing.T) {
	paths := []string{}
	service := recordApprovalTargetService("eventHint", "부산 공급사 미팅", "event-1")(t, &paths)

	response := resolveApprovalTargetThroughRoute(t, service, "message_send", json.RawMessage(`{"targetType":"currentThread","message":"안녕하세요"}`))

	if response.IsError {
		t.Fatalf("a tool with nothing to resolve ahead is not a failure, got %+v", response)
	}
	if decodeResolvedApprovalTarget(t, response).ID != "" {
		t.Fatalf("expected no target, got %s", response.Result)
	}
	if len(paths) != 0 {
		t.Fatalf("a tool with no target resolver reaches no backend, got %+v", paths)
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
			if errorValue := capabilityschema.ValidateInput(descriptor.InputSchema, narrowedInput); errorValue != nil {
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
	if len(capabilityToolTargetRoutes) != len(previewApprovalTargetRouteNames) {
		t.Fatalf("every route that resolves a target ahead needs the round trip proven, got %d routes and %d fixtures", len(capabilityToolTargetRoutes), len(previewApprovalTargetRouteNames))
	}
	for _, route := range capabilityToolTargetRoutes {
		isCovered := false
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
	paths := []string{}
	service := recordApprovalTargetService("eventHint", "부산 공급사 미팅", "event-1")(t, &paths)
	requestBody := `{"input":{"eventHint":"부산 공급사 미팅"},"context":{"requesterEmail":"member@example.com"}}`
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

func TestTheRecordsDestructiveToolsResolveTheirTargetThroughTheRecord(t *testing.T) {
	carrier := reflect.ValueOf(Service.previewRecordToolTarget).Pointer()
	for _, fixture := range approvalTargetFixtures {
		route, hasRoute := capabilityToolTargetRouteFor(fixture.toolName)
		if !hasRoute {
			t.Errorf("%s resolves no target", fixture.toolName)
			continue
		}
		if reflect.ValueOf(route.Resolver).Pointer() != carrier {
			t.Errorf("%s is answered by the record and previews its target somewhere else", fixture.toolName)
		}
	}
}
