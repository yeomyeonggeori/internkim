package companion

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/anthropic-lab/internkim/internal/capabilities"
)

type fakeApprovalHandler struct {
	decision ApprovalDecision
	calls    int
}

func (handler *fakeApprovalHandler) Approve(ctx context.Context, request ApprovalRequest) (ApprovalDecision, error) {
	_ = ctx
	_ = request
	handler.calls++
	return handler.decision, nil
}

func TestCompanionRequestSigningRoundTrip(t *testing.T) {
	keyPair, errorValue := GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	body := []byte(`{"status":"ok"}`)
	request, errorValue := http.NewRequest(http.MethodPost, "https://device.example.test/_internkim/companion/heartbeat", bytes.NewReader(body))
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := SignRequest(request, body, keyPair.PrivateKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !VerifyRequestSignature(request, body, keyPair.PublicKey) {
		t.Fatal("expected signed request to verify")
	}
	if VerifyRequestSignature(request, []byte(`{"status":"tampered"}`), keyPair.PublicKey) {
		t.Fatal("expected tampered body to fail verification")
	}
}

func TestGrantStoreReusesApprovedBrowserGrant(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{Allowed: true}}
	request := capabilities.ToolInvokeRequest{
		ToolName:      "browser.navigate",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	envelope := JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.navigate",
		ResourceScope: request.ResourceScope,
	}

	if errorValue := store.Authorize(context.Background(), envelope, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected first approval to succeed: %v", errorValue)
	}
	nextEnvelope := envelope
	nextEnvelope.JobID = "job-2"
	if errorValue := store.Authorize(context.Background(), nextEnvelope, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected grant reuse to succeed: %v", errorValue)
	}
	if approvalHandler.calls != 1 {
		t.Fatalf("expected one approval call, got %d", approvalHandler.calls)
	}
}

func TestGrantStoreDeniesWithReason(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{
		Allowed:             false,
		UserReason:          "not this site",
		SuggestedConstraint: "ask me for text",
	}}
	request := capabilities.ToolInvokeRequest{
		ToolName:      "browser.navigate",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://bank.example"},
	}
	envelope := JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.navigate",
		ResourceScope: request.ResourceScope,
	}

	errorValue := store.Authorize(context.Background(), envelope, request, approvalHandler)
	denialError, ok := errorValue.(DenialError)
	if !ok {
		t.Fatalf("expected denial error, got %v", errorValue)
	}
	if denialError.Denial.UserReason != "not this site" || denialError.Denial.SuggestedConstraint != "ask me for text" {
		t.Fatalf("unexpected denial: %+v", denialError.Denial)
	}
}

func TestUserInputDoesNotNeedGrant(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{Allowed: false}}
	request := capabilities.ToolInvokeRequest{ToolName: "user.input"}

	if errorValue := store.Authorize(context.Background(), JobEnvelope{JobID: "job-1", ToolName: "user.input"}, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected user input to bypass grants: %v", errorValue)
	}
	if approvalHandler.calls != 0 {
		t.Fatalf("expected approval not to be called, got %d", approvalHandler.calls)
	}
}
