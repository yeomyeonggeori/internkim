package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const twoSamplePeople = `{"people":[` +
	`{"memberID":"person-2","email":"departing@example.com","name":"박예시"},` +
	`{"memberID":"person-3","email":"another@example.com","name":"박예시"},` +
	`{"memberID":"person-4","email":"staying@example.com","name":"이샘플"}]}`

func personMessageDeleteCall(personHint string) capabilities.ToolInvokeRequest {
	input, _ := json.Marshal(map[string]string{personHintField: personHint})
	return capabilities.ToolInvokeRequest{
		ToolName: personMessageDeleteToolName,
		Input:    input,
		Context:  capabilities.ToolInvokeContext{RequesterEmail: "admin@example.com"},
	}
}

func reachingTheDirectoryAndTheWipe(wipe *standingIn) map[gateBackend]*standingIn {
	return map[gateBackend]*standingIn{
		admindOverHTTP:      answering(twoSamplePeople),
		admindOverTheSocket: wipe,
	}
}

func TestThePersonMessageDeleteTargetNamesWhoTheCallReaches(t *testing.T) {
	service := serviceReaching(t, reachingTheDirectoryAndTheWipe(answering(`{}`)))

	byName, _ := service.resolvePersonMessageDeleteTarget(context.Background(), personMessageDeleteCall("이샘플"))
	target := decodeResolvedApprovalTarget(t, byName)
	if target.InputField != personHintField || target.ID != "person-4" || target.Title != "이샘플 <staying@example.com>" {
		t.Fatalf("the approval question has to name the person whose messages go, got %+v", target)
	}

	byIdentity, _ := service.resolvePersonMessageDeleteTarget(context.Background(), personMessageDeleteCall(target.ID))
	if narrowed := decodeResolvedApprovalTarget(t, byIdentity); !reflect.DeepEqual(narrowed, target) {
		t.Fatalf("the approved call is replayed with the identity, which has to reach the same person, got %+v", narrowed)
	}
}

func TestDeletingAPersonsMessagesAsksTheHostAsTheRequesterForTheResolvedAddress(t *testing.T) {
	wipe := answering(`{"email":"staying@example.com","buzzDeleted":3,"mattermostDeleted":0}`)
	service := serviceReaching(t, reachingTheDirectoryAndTheWipe(wipe))

	answered, errorValue := service.invokePersonMessageDeleteTool(context.Background(), personMessageDeleteCall("staying@example.com"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectSucceeded(t, answered)
	expectResultHolds(t, answered, `"deletedCount":3`)
	if len(wipe.asked) != 1 || wipe.asked[0].Path != admindPersonMessageWipePath || !strings.Contains(wipe.asked[0].Body, `"email":"staying@example.com"`) {
		t.Fatalf("the host deletes what the named person wrote, got %+v", wipe.asked)
	}
}

func TestANameTwoPeopleCarryDeletesNothingAndAsksWhich(t *testing.T) {
	wipe := answering(`{}`)
	service := serviceReaching(t, reachingTheDirectoryAndTheWipe(wipe))

	answered, errorValue := service.invokePersonMessageDeleteTool(context.Background(), personMessageDeleteCall("박예시"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !answered.IsError || answered.ErrorCode != "interaction_required" {
		t.Fatalf("an ambiguous person is a question for the requester, got %+v", answered)
	}
	if !strings.Contains(string(answered.Result), "departing@example.com") || !strings.Contains(string(answered.Result), "another@example.com") {
		t.Fatalf("the question names each candidate by address, got %s", answered.Result)
	}
	if len(wipe.asked) != 0 {
		t.Fatalf("nothing is deleted while it is unclear whose messages go, got %+v", wipe.asked)
	}
}

func TestTheHostsRefusalOfANonAdministratorReachesTheCaller(t *testing.T) {
	refusing := answeringPerCall(func(*http.Request) (int, string) { return http.StatusForbidden, "admin access required" })
	service := serviceReaching(t, reachingTheDirectoryAndTheWipe(refusing))

	answered, errorValue := service.invokePersonMessageDeleteTool(context.Background(), personMessageDeleteCall("이샘플"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !answered.IsError || answered.ErrorCode != "not_allowed" || !strings.Contains(answered.Message, "admin access required") {
		t.Fatalf("the host decides who may delete, and its answer is the caller's, got %+v", answered)
	}
}
