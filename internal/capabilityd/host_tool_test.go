package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

type admindStandIn struct {
	status int
	answer string
	paths  []string
	bodies []string
	emails []string
}

func serviceAskingAdmind(admind *admindStandIn) Service {
	return Service{HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(request.Body)
		admind.paths = append(admind.paths, request.URL.Path)
		admind.bodies = append(admind.bodies, string(body))
		admind.emails = append(admind.emails, request.Header.Get(admindRequesterEmailHeader))
		return &http.Response{StatusCode: admind.status, Body: io.NopCloser(strings.NewReader(admind.answer)), Header: http.Header{"Content-Type": {"application/json"}}}, nil
	})}}
}

func hostUpdateCall(input string, toolContext capabilityprotocol.ToolInvokeContext) capabilities.ToolInvokeRequest {
	toolContext.RequesterEmail = "member1@example.com"
	return capabilities.ToolInvokeRequest{ToolName: hostUpdateToolName, Input: json.RawMessage(input), Context: toolContext}
}

func carriedApproval(t *testing.T, body string) bool {
	t.Helper()
	var carried hostUpdateStart
	if errorValue := json.Unmarshal([]byte(body), &carried); errorValue != nil {
		t.Fatal(errorValue)
	}
	return carried.IsApproved
}

func TestTheHostUpdateTellsAdmindWhetherTheRequesterApprovedThisCall(t *testing.T) {
	input := `{"targetVersion":"v2026.10.02.090000"}`
	scheduledCall := func(toolName string, toolInput string) *capabilityprotocol.ScheduledApprovedCall {
		return &capabilityprotocol.ScheduledApprovedCall{ToolName: toolName, ToolInput: json.RawMessage(toolInput), ApproverPersonID: "person-1"}
	}
	for _, testCase := range []struct {
		name       string
		context    capabilityprotocol.ToolInvokeContext
		isApproved bool
	}{
		{"an unapproved call", capabilityprotocol.ToolInvokeContext{}, false},
		{"a scheduled run that carries no approved call", capabilityprotocol.ToolInvokeContext{IsScheduledRun: true}, false},
		{"a held call spent", capabilityprotocol.ToolInvokeContext{HoldID: "held-1"}, true},
		{"the scheduled run of exactly this call", capabilityprotocol.ToolInvokeContext{IsScheduledRun: true, ScheduledApprovedCall: scheduledCall("host_update", `{ "targetVersion": "v2026.10.02.090000" }`)}, true},
		{"a scheduled run that approved another tool", capabilityprotocol.ToolInvokeContext{IsScheduledRun: true, ScheduledApprovedCall: scheduledCall("message_send", input)}, false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			admind := &admindStandIn{status: http.StatusForbidden, answer: `{"errorCode":"approval_required","message":"host_update requires the requester's approval before it runs"}`}
			toolContext := testCase.context
			toolContext.ConversationID = "conversation-1"
			if _, errorValue := serviceAskingAdmind(admind).invokeHostUpdateTool(context.Background(), hostUpdateCall(input, toolContext)); errorValue != nil {
				t.Fatal(errorValue)
			}
			if admind.paths[0] != admindHostUpdatePath || admind.emails[0] != "member1@example.com" {
				t.Fatalf("the update reached %s as %s", admind.paths[0], admind.emails[0])
			}
			if got := carriedApproval(t, admind.bodies[0]); got != testCase.isApproved {
				t.Fatalf("admind was told isApproved=%v", got)
			}
			if !strings.Contains(admind.bodies[0], `"conversationID":"conversation-1"`) {
				t.Fatalf("admind was not told where to report: %s", admind.bodies[0])
			}
		})
	}
}

func TestTheHostUpdateTargetCarriesAdmindsFactsAndChoices(t *testing.T) {
	admind := &admindStandIn{status: http.StatusOK, answer: `{"inputField":"targetVersion","id":"v2026.10.02.090000","title":"v2026.10.01.000000 → v2026.10.02.090000","preview":"{\"expectedDowntimeSeconds\":60}","choices":[{"key":"offHours","startsAt":"2026-10-03T03:00:00+09:00"},{"key":"now"}]}`}
	response, errorValue := serviceAskingAdmind(admind).resolveCapabilityToolTarget(context.Background(), hostUpdateToolName, strings.NewReader(`{"toolName":"host_update","input":{},"context":{"requesterEmail":"member1@example.com"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if admind.paths[0] != admindHostUpdatePlanURL || admind.emails[0] != "member1@example.com" {
		t.Fatalf("the plan was asked of %s as %s", admind.paths[0], admind.emails[0])
	}
	if response.IsError || response.Status != "resolved" || !strings.Contains(string(response.Result), `"choices":[{"key":"offHours"`) || decodeResolvedApprovalTarget(t, response).ID != "v2026.10.02.090000" {
		t.Fatalf("the target came back as %+v", response)
	}
}

func TestAHostUpdateAdmindRefusesComesBackAsAFailureBeforeAnyQuestion(t *testing.T) {
	admind := &admindStandIn{status: http.StatusForbidden, answer: `{"errorCode":"access_denied","failureStage":"authorization","message":"only an administrator of this company can update its host"}`}
	response, errorValue := serviceAskingAdmind(admind).resolveCapabilityToolTarget(context.Background(), hostUpdateToolName, strings.NewReader(`{"toolName":"host_update","input":{},"context":{"requesterEmail":"member2@example.com"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.IsError || response.ErrorCode != "access_denied" || !strings.Contains(response.Message, "administrator") {
		t.Fatalf("a refused plan came back as %+v", response)
	}
}

func publicAPIHostUpdate(input string) string {
	return `{"toolName":"host_update","input":` + input + `,"context":{"requesterPersonID":"person-1","requesterEmail":"member1@example.com","taskSource":"public_api","holdID":"held-test"}}`
}

func TestAHostUpdateCalledThroughThePublicAPIStartsAsTheCallerAsked(t *testing.T) {
	admind := &admindStandIn{status: http.StatusOK, answer: `{"status":"started","fromVersion":"v2026.10.01.000000","toVersion":"v2026.10.02.090000","startedAt":"2026-10-02T14:00:00Z","expectedDowntimeSeconds":60}`}
	response, errorValue := serviceAskingAdmind(admind).invokeCapabilityTool(context.Background(), hostUpdateToolName, strings.NewReader(publicAPIHostUpdate(`{}`)))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || len(admind.paths) != 1 || admind.emails[0] != "member1@example.com" || !carriedApproval(t, admind.bodies[0]) {
		t.Fatalf("the update came back as %+v after admind heard %v", response, admind.bodies)
	}
}

func TestAHostUpdateCalledThroughThePublicAPIIsNotHeldForLater(t *testing.T) {
	admind := &admindStandIn{status: http.StatusOK, answer: `{}`}
	response, errorValue := serviceAskingAdmind(admind).invokeCapabilityTool(context.Background(), hostUpdateToolName, strings.NewReader(publicAPIHostUpdate(`{"startsAt":"2026-10-03T03:00:00+09:00"}`)))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(admind.paths) != 0 || response.ErrorCode != "scheduled_start_unsupported" || !strings.Contains(response.Message, "leave startsAt out") {
		t.Fatalf("a held start came back as %+v after admind heard %v", response, admind.paths)
	}
}

func TestAnAgentsHostUpdateWithAStartTimeStillReachesAdmindOnceApproved(t *testing.T) {
	admind := &admindStandIn{status: http.StatusOK, answer: `{"status":"started","fromVersion":"v2026.10.01.000000","toVersion":"v2026.10.02.090000","startedAt":"2026-10-02T14:00:00Z","expectedDowntimeSeconds":60}`}
	call := hostUpdateCall(`{"startsAt":"2026-10-03T03:00:00+09:00"}`, capabilityprotocol.ToolInvokeContext{HoldID: "held-1", ConversationID: "conversation-1"})
	response, errorValue := serviceAskingAdmind(admind).invokeHostUpdateTool(context.Background(), call)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.IsError || len(admind.paths) != 1 {
		t.Fatalf("the agent's approved update came back as %+v", response)
	}
}

func TestAnAgentsHostUpdateStillWaitsForTheRequestersApproval(t *testing.T) {
	admind := &admindStandIn{status: http.StatusOK, answer: `{}`}
	response, errorValue := serviceAskingAdmind(admind).invokeCapabilityTool(context.Background(), hostUpdateToolName, strings.NewReader(`{"toolName":"host_update","input":{},"context":{"requesterPersonID":"person-1","requesterEmail":"member1@example.com","conversationID":"conversation-1"}}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(admind.paths) != 0 || response.ErrorCode != "approval_required" {
		t.Fatalf("an unapproved agent call came back as %+v after admind heard %v", response, admind.paths)
	}
}
