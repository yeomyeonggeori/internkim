package jsonschema

import (
	"encoding/json"
	"strings"
	"testing"
)

const taskContract = `{
	"type":"object",
	"additionalProperties":false,
	"required":["taskID","tasks"],
	"properties":{
		"taskID":{"type":"string"},
		"scope":{"type":"string","enum":["self","all"]},
		"tasks":{"type":"array","items":{
			"type":"object",
			"additionalProperties":false,
			"required":["id"],
			"properties":{"id":{"type":"string"},"title":{"type":"string"}}
		}}
	}
}`

func TestInputRejectionNamesWhatWasWrongAndWhatIsAccepted(t *testing.T) {
	errorValue := ValidateInput(json.RawMessage(taskContract), json.RawMessage(`{"scope":"everyone","tasks":[]}`))
	if errorValue == nil {
		t.Fatal("expected an enum outside the schema to fail")
	}
	message := errorValue.Error()
	for _, expected := range []string{
		`input.scope must be one of "self", "all", and it is "everyone"`,
		"This tool takes: scope, taskID, tasks",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected the rejection to carry %q, got %q", expected, message)
		}
	}
}

// One call is all a model gets to recover with, so a rejection that stops at
// the first fault sends it back for the second one.
func TestInputRejectionNamesEveryFieldAtOnce(t *testing.T) {
	errorValue := ValidateInput(json.RawMessage(taskContract), json.RawMessage(`{"taskID":7,"scope":"everyone","tasks":{}}`))
	if errorValue == nil {
		t.Fatal("expected three faults to fail")
	}
	message := errorValue.Error()
	for _, expected := range []string{
		"input.taskID must be a string, and it is 7",
		`input.scope must be one of "self", "all", and it is "everyone"`,
		"input.tasks must be an array, and it is {}",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected the rejection to carry %q, got %q", expected, message)
		}
	}
}

func TestInputRejectionNamesTheMissingField(t *testing.T) {
	errorValue := ValidateInput(json.RawMessage(taskContract), json.RawMessage(`{"tasks":[]}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "taskID") {
		t.Fatalf("expected the missing field to be named, got %v", errorValue)
	}
}

// A newer plane answers with fields the device's contract has never heard of,
// and the two are deployed on their own schedules. Refusing the answer takes
// the tool out of service until the fleet is updated; carrying it costs nothing
// because every reader takes fields by name.
func TestResultCarriesFieldsTheContractDoesNotKnow(t *testing.T) {
	answer := json.RawMessage(`{
		"taskID":"t-1",
		"organizationID":"o-1",
		"tasks":[{"id":"t-1","opportunityID":"p-1"}]
	}`)

	check, errorValue := ValidateResult(json.RawMessage(taskContract), answer)
	if errorValue != nil {
		t.Fatalf("expected unknown fields to be carried: %v", errorValue)
	}
	if strings.Join(check.UnknownFields, " ") != "organizationID tasks[].opportunityID" {
		t.Fatalf("expected the unknown fields to be named, got %v", check.UnknownFields)
	}
}

func TestResultStillHoldsWhatTheContractPromises(t *testing.T) {
	broken := map[string]json.RawMessage{
		"missing required":     json.RawMessage(`{"tasks":[]}`),
		"wrong type":           json.RawMessage(`{"taskID":7,"tasks":[]}`),
		"value outside enum":   json.RawMessage(`{"taskID":"t-1","scope":"everyone","tasks":[]}`),
		"broken nested member": json.RawMessage(`{"taskID":"t-1","tasks":[{"title":"no id"}]}`),
	}
	for name, answer := range broken {
		check, errorValue := ValidateResult(json.RawMessage(taskContract), answer)
		if errorValue == nil {
			t.Fatalf("%s: expected the contract to refuse the answer, carried %v", name, check.UnknownFields)
		}
		if !strings.Contains(errorValue.Error(), "tool result does not match") {
			t.Fatalf("%s: expected a result-side message, got %q", name, errorValue.Error())
		}
	}
}

// The run this was written for read `validating /properties/tasks: type: null
// has type "null", want "array"` and spent every remaining call rewriting an
// input that was already correct. The answer is the tool's, and the message has
// to say so.
func TestResultRejectionNamesTheFieldAndAbsolvesTheCall(t *testing.T) {
	_, errorValue := ValidateResult(json.RawMessage(taskContract), json.RawMessage(`{"taskID":"t-1","tasks":null}`))
	if errorValue == nil {
		t.Fatal("expected a null where an array is promised to fail")
	}
	message := errorValue.Error()
	for _, expected := range []string{
		"result.tasks is required and is missing",
		"the same request with different arguments will not change it",
	} {
		if !strings.Contains(message, expected) {
			t.Fatalf("expected the rejection to carry %q, got %q", expected, message)
		}
	}
	if strings.Contains(message, "/properties/") {
		t.Fatalf("expected the field named where the caller reads it, got %q", message)
	}
}

func TestResultRejectionNamesTheMemberThatBrokeIt(t *testing.T) {
	_, errorValue := ValidateResult(json.RawMessage(taskContract), json.RawMessage(`{"taskID":"t-1","tasks":[{"id":"t-1"},{"title":"no id"}]}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "result.tasks[1].id is required and is missing") {
		t.Fatalf("expected the failing member to be named by index, got %v", errorValue)
	}
}

func TestResultKeepsQuietWhenTheAnswerFits(t *testing.T) {
	check, errorValue := ValidateResult(json.RawMessage(taskContract), json.RawMessage(`{"taskID":"t-1","tasks":[{"id":"t-1"}]}`))
	if errorValue != nil {
		t.Fatalf("expected a contracted answer to pass: %v", errorValue)
	}
	if len(check.UnknownFields) != 0 {
		t.Fatalf("expected nothing to report, got %v", check.UnknownFields)
	}
}
