package companion

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type fakeApprovalHandler struct {
	decision    ApprovalDecision
	lastRequest ApprovalRequest
	calls       int
}

func (handler *fakeApprovalHandler) Approve(ctx context.Context, request ApprovalRequest) (ApprovalDecision, error) {
	_ = ctx
	handler.lastRequest = request
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
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	envelope := JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.open",
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

func TestGrantStoreListsAndRevokesActiveGrant(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{Allowed: true}}
	request := capabilities.ToolInvokeRequest{
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	envelope := JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.open",
		ResourceScope: request.ResourceScope,
	}

	if errorValue := store.Authorize(context.Background(), envelope, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected approval to create grant: %v", errorValue)
	}
	grants := store.ListActive()
	if len(grants) != 1 {
		t.Fatalf("expected one active grant, got %d", len(grants))
	}
	if !store.Revoke(grants[0].GrantID) {
		t.Fatal("expected revoke to succeed")
	}
	if len(store.ListActive()) != 0 {
		t.Fatal("expected revoked grant to disappear from active list")
	}
	nextEnvelope := envelope
	nextEnvelope.JobID = "job-2"
	if errorValue := store.Authorize(context.Background(), nextEnvelope, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected new approval after revoke: %v", errorValue)
	}
	if approvalHandler.calls != 2 {
		t.Fatalf("expected approval to be requested again, got %d", approvalHandler.calls)
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
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://bank.example"},
	}
	envelope := JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.open",
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

func TestGrantStoreRemembersCapabilityForRestOfTaskOnly(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{Allowed: true, RememberSession: true}}
	request := capabilities.ToolInvokeRequest{ToolName: "browser.handoff"}
	envelope := JobEnvelope{JobID: "job-1", ToolName: "browser.handoff"}

	if errorValue := store.Authorize(context.Background(), envelope, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected approval to create session grant: %v", errorValue)
	}

	sameTaskRequest := capabilities.ToolInvokeRequest{
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://console.cloud.google.com"},
	}
	sameTaskEnvelope := JobEnvelope{
		JobID:         "job-2",
		ParentJobID:   "job-1",
		ToolName:      "browser.open",
		ResourceScope: sameTaskRequest.ResourceScope,
	}
	if errorValue := store.Authorize(context.Background(), sameTaskEnvelope, sameTaskRequest, approvalHandler); errorValue != nil {
		t.Fatalf("expected session grant reuse within the same task: %v", errorValue)
	}
	if approvalHandler.calls != 1 {
		t.Fatalf("expected reuse within the same task to skip approval, got %d calls", approvalHandler.calls)
	}

	unrelatedTaskRequest := capabilities.ToolInvokeRequest{
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://console.cloud.google.com"},
	}
	unrelatedTaskEnvelope := JobEnvelope{
		JobID:         "job-99",
		ToolName:      "browser.open",
		ResourceScope: unrelatedTaskRequest.ResourceScope,
	}
	if errorValue := store.Authorize(context.Background(), unrelatedTaskEnvelope, unrelatedTaskRequest, approvalHandler); errorValue != nil {
		t.Fatalf("expected an unrelated task to be approved independently: %v", errorValue)
	}
	if approvalHandler.calls != 2 {
		t.Fatalf("expected a job outside the granting task to require a fresh approval, got %d calls", approvalHandler.calls)
	}

	grants := store.ListActive()
	if len(grants) != 2 {
		t.Fatalf("expected the original task grant and the new unrelated task grant to both remain active, got %d", len(grants))
	}
	displayNamesByAnchor := map[string]string{}
	for _, grant := range grants {
		displayNamesByAnchor[grant.AnchorJobID] = grant.DisplayName
	}
	if displayNamesByAnchor["job-1"] != "Browser access to this session" {
		t.Fatalf("unexpected display name for the original task grant: %s", displayNamesByAnchor["job-1"])
	}
	if displayNamesByAnchor["job-99"] != "Browser access to https://console.cloud.google.com" {
		t.Fatalf("unexpected display name for the unrelated task grant: %s", displayNamesByAnchor["job-99"])
	}
}

func TestGrantStoreRejectsReuseAcrossUnrelatedTasks(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{Allowed: true, RememberSession: true}}

	taskARequest := capabilities.ToolInvokeRequest{ToolName: "browser.handoff"}
	taskAEnvelope := JobEnvelope{JobID: "task-a-root", ToolName: "browser.handoff"}
	if errorValue := store.Authorize(context.Background(), taskAEnvelope, taskARequest, approvalHandler); errorValue != nil {
		t.Fatalf("expected task A approval to create a session grant: %v", errorValue)
	}

	taskBRequest := capabilities.ToolInvokeRequest{
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	taskBEnvelope := JobEnvelope{
		JobID:         "task-b-root",
		ToolName:      "browser.open",
		ResourceScope: taskBRequest.ResourceScope,
	}
	if errorValue := store.Authorize(context.Background(), taskBEnvelope, taskBRequest, approvalHandler); errorValue != nil {
		t.Fatalf("expected task B to be approved on its own: %v", errorValue)
	}
	if approvalHandler.calls != 2 {
		t.Fatalf("expected a job from an unrelated task to require its own approval despite matching capability, got %d calls", approvalHandler.calls)
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

func TestApprovalRequestIncludesContextDeadline(t *testing.T) {
	store := NewMemoryGrantStore()
	approvalHandler := &fakeApprovalHandler{decision: ApprovalDecision{Allowed: true}}
	request := capabilities.ToolInvokeRequest{
		ToolName:      "browser.open",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	envelope := JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser.open",
		ResourceScope: request.ResourceScope,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	if errorValue := store.Authorize(ctx, envelope, request, approvalHandler); errorValue != nil {
		t.Fatalf("expected approval to succeed: %v", errorValue)
	}
	if approvalHandler.lastRequest.TimeoutSeconds < 1 || approvalHandler.lastRequest.TimeoutSeconds > 2 {
		t.Fatalf("expected approval timeout from context deadline, got %+v", approvalHandler.lastRequest)
	}
}
