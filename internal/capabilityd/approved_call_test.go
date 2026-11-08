package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
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

func TestACallTheAPITokenOrThePlaneMakesRunsOnTheirAuthority(t *testing.T) {
	for _, taskSource := range []string{capabilities.TaskSourcePublicAPI, capabilities.TaskSourcePlaneTelling} {
		assertCallIsAllowed(t,
			capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", TaskSource: taskSource},
			taskSource+" is an explicit act checked before the call reaches the device")
	}
}

func TestAScheduledRunCarriesNoApprovalOfItsOwn(t *testing.T) {
	assertCallIsRefused(t, capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", IsScheduledRun: true})
}

func TestACallNamingAnApprovalRunsOnlyWhenTheLedgerRedeemsIt(t *testing.T) {
	for _, testCase := range []struct {
		ledgerStatus int
		isDenied     bool
	}{
		{http.StatusOK, false},
		{http.StatusConflict, true},
	} {
		ledger := &blueclawLedgerAnswering{status: testCase.ledgerStatus}
		service := Service{Configuration: Configuration{BlueclawBaseURL: ledger.serve(t)}}
		request := aCallToApprove(capabilities.ToolInvokeContext{RequesterPersonID: "person-sample", HoldID: "held-4f2a91c0", TaskRunID: "run-1"})

		_, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptorForCapabilityToolTest(t, request.ToolName))

		if isDenied != testCase.isDenied {
			t.Fatalf("with the ledger answering %d, the call was denied=%v", testCase.ledgerStatus, isDenied)
		}
	}
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
