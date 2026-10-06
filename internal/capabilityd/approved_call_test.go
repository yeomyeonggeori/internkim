package capabilityd

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

func aCallToApprove(toolContext capabilities.ToolInvokeContext) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"박예시","message":"확인 부탁드립니다"}`),
		Context:  toolContext,
	}
}

func assertCallIsAllowed(t *testing.T, toolContext capabilities.ToolInvokeContext, why string) {
	t.Helper()
	request := aCallToApprove(toolContext)
	response, isDenied := Service{}.capabilityToolApprovalDeniedResponse(
		context.Background(),
		request,
		descriptorForCapabilityToolTest(t, request.ToolName),
	)
	if isDenied {
		t.Fatalf("%s, and it was refused: %+v", why, response)
	}
}

func scheduledRunApproving(toolName string, toolInput string) capabilities.ToolInvokeContext {
	return capabilities.ToolInvokeContext{
		RequesterPersonID:     "person-sample",
		IsScheduledRun:        true,
		ScheduledApprovedCall: &capabilityprotocol.ScheduledApprovedCall{ToolName: toolName, ToolInput: json.RawMessage(toolInput), ApproverPersonID: "person-sample"},
	}
}

func TestAScheduledRunOfExactlyTheApprovedCallRuns(t *testing.T) {
	call := aCallToApprove(capabilities.ToolInvokeContext{})
	assertCallIsAllowed(t,
		scheduledRunApproving(call.ToolName, ` {"message":"확인 부탁드립니다", "personHint":"박예시", "targetType":"directMessage"} `),
		"the requester approved this exact call when they deferred it")
}

func TestACallTheAPITokenOrThePlaneMakesRunsOnTheirAuthority(t *testing.T) {
	for _, taskSource := range []string{capabilities.TaskSourcePublicAPI, capabilities.TaskSourcePlaneTelling} {
		assertCallIsAllowed(t,
			capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", TaskSource: taskSource},
			taskSource+" is an explicit act checked before the call reaches the device")
	}
}

func TestAScheduledRunWithNoApprovedCallIsRefused(t *testing.T) {
	assertCallIsRefused(t, capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", IsScheduledRun: true})
}

func TestAScheduledRunOfADifferentCallThanTheApprovedOneIsRefused(t *testing.T) {
	call := aCallToApprove(capabilities.ToolInvokeContext{})
	assertCallIsRefused(t, scheduledRunApproving(call.ToolName, `{"targetType":"directMessage","personHint":"박예시","message":"다른 내용"}`))
	assertCallIsRefused(t, scheduledRunApproving("mail_send", string(call.Input)))
}

func TestAnApprovedCallOnARunThatIsNotScheduledIsRefused(t *testing.T) {
	call := aCallToApprove(capabilities.ToolInvokeContext{})
	toolContext := scheduledRunApproving(call.ToolName, string(call.Input))
	toolContext.IsScheduledRun = false
	assertCallIsRefused(t, toolContext)
}

func TestACallNamingTheApprovalItSpendsRuns(t *testing.T) {
	assertCallIsAllowed(t,
		capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", HoldID: "held-4f2a91c0"},
		"a call approved inside its own turn names the held call it spends")
}

func assertCallIsRefused(t *testing.T, toolContext capabilities.ToolInvokeContext) {
	t.Helper()
	request := aCallToApprove(toolContext)
	_, isDenied := Service{}.capabilityToolApprovalDeniedResponse(
		context.Background(),
		request,
		descriptorForCapabilityToolTest(t, request.ToolName),
	)
	if !isDenied {
		t.Fatalf("a call nobody approved ran: %+v", toolContext)
	}
}

func TestACallClaimingNoApprovalIsStillRefused(t *testing.T) {
	request := aCallToApprove(capabilities.ToolInvokeContext{RequesterPersonID: "person-sample"})

	response, isDenied := Service{}.capabilityToolApprovalDeniedResponse(
		context.Background(),
		request,
		descriptorForCapabilityToolTest(t, request.ToolName),
	)

	if !isDenied {
		t.Fatal("a call nobody approved ran, so the approval gate decides nothing")
	}
	assertCapabilityApprovalRequired(t, response, "message_send")
}
