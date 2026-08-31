package admind

import (
	"encoding/json"
	"testing"

	capabilities "gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestAPublicFailureSaysEachThingOnce(t *testing.T) {
	answer := publicToolInvokeAnswer(capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        "task_update",
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		IsError:         true,
		Content:         "no task matched taskHint",
		Message:         "no task matched taskHint",
		ErrorCode:       "task_hint_unresolved",
		FailureStage:    "target_resolution",
		Retryable:       true,
		SafeRetry:       true,
		Result:          json.RawMessage(`{"message":"no task matched taskHint","errorCode":"task_hint_unresolved","failureStage":"target_resolution","retryable":true,"safeRetry":true,"candidates":[{"taskID":"t1"}]}`),
	})

	for _, gone := range []string{"provider", "status", "isError", "content", "safeRetry"} {
		if _, found := answer[gone]; found {
			t.Fatalf("%s is a second copy and must not reach a public caller", gone)
		}
	}
	if answer["message"] != "no task matched taskHint" || answer["errorCode"] != "task_hint_unresolved" {
		t.Fatalf("the one copy is missing: %v", answer)
	}
	var result map[string]json.RawMessage
	if errorValue := json.Unmarshal(answer["result"].(json.RawMessage), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found := result["candidates"]; !found {
		t.Fatalf("the failure's own payload must survive: %v", result)
	}
	for _, gone := range []string{"message", "errorCode", "failureStage", "retryable", "safeRetry"} {
		if _, found := result[gone]; found {
			t.Fatalf("result still repeats %s", gone)
		}
	}
}

func TestAPublicSuccessKeepsItsResultUntouched(t *testing.T) {
	document := json.RawMessage(`{"count":1,"message":"a field that belongs to the tool"}`)
	answer := publicToolInvokeAnswer(capabilities.ToolInvokeResponse{
		SelectedBackend: "device",
		ToolName:        "task_list",
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "ok",
		Result:          document,
	})

	if string(answer["result"].(json.RawMessage)) != string(document) {
		t.Fatalf("a success result is the tool's own document: %v", answer)
	}
	if _, found := answer["retryable"]; found {
		t.Fatal("retryable is a failure fact")
	}
}

func TestAResultFieldThatMerelySharesANameIsKept(t *testing.T) {
	answer := publicToolInvokeAnswer(capabilities.ToolInvokeResponse{
		ToolName:  "message_send",
		Outcome:   capabilities.ToolOutcomeFailed,
		Message:   "the channel refused the post",
		ErrorCode: "post_refused",
		Result:    json.RawMessage(`{"message":"the text the caller tried to send"}`),
	})

	var result map[string]string
	if errorValue := json.Unmarshal(answer["result"].(json.RawMessage), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result["message"] != "the text the caller tried to send" {
		t.Fatalf("a same-named field with its own value was destroyed: %v", result)
	}
}
