package capabilityd

import (
	"context"
	"encoding/json"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
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

func TestATurnCarryingOutAnEarlierApprovalRuns(t *testing.T) {
	assertCallIsAllowed(t,
		capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", IsApprovalContinuation: true},
		"a continuation turn says the requester approved this call")
}

func TestACallNamingTheApprovalItSpendsRuns(t *testing.T) {
	assertCallIsAllowed(t,
		capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", ApprovedCallID: "held-4f2a91c0"},
		"a call approved inside its own turn names the held call it spends")
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
