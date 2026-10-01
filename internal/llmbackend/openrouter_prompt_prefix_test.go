package llmbackend

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func turnMessages(systemInstruction string, finalUserMessage string) []Message {
	return []Message{
		{Role: "system", Content: systemInstruction},
		{Role: "user", Content: "open the attendance board"},
		{Role: "assistant", Content: "the board is open"},
		{Role: "user", Content: finalUserMessage},
	}
}

func agentTurnSchema(t *testing.T) StructuredOutputSchema {
	t.Helper()
	return StructuredOutputSchema{
		Name: "bluecollar_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, []capabilities.Descriptor{
			{Name: "write", InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)},
			{Name: "attendance_summary", InputSchema: json.RawMessage(`{"type":"object","properties":{"month":{"type":"string"}},"required":["month"]}`)},
			{Name: "platform_message", InputSchema: json.RawMessage(`{"type":"object","properties":{"conversationID":{"type":"string"},"text":{"type":"string"}},"required":["conversationID","text"]}`)},
		}),
	}
}

func actionTurnDocument(t *testing.T, backend OpenRouterBackend, schema StructuredOutputSchema, messages []Message) []byte {
	t.Helper()
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(schema)
	if errorValue != nil || !isActionSchema {
		t.Fatalf("expected an agent turn schema: %v", errorValue)
	}
	request := StructuredRequest{SessionID: "task-run-4a2", Messages: messages, StructuredOutputSchema: schema}
	document, _, errorValue := backend.buildChatActionRequest(request, "a-model", toolSet.Tools, toolSet.NativeSchemaLint)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func structuredTurnDocument(t *testing.T, backend OpenRouterBackend, schema StructuredOutputSchema, messages []Message) []byte {
	t.Helper()
	request := StructuredRequest{SessionID: "task-run-4a2", Messages: messages, StructuredOutputSchema: schema}
	document, errorValue := json.Marshal(backend.buildStructuredRequest(request, "a-model").Document)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func prefixBeforeFinalMessage(t *testing.T, document []byte, messages []Message) []byte {
	t.Helper()
	openAIMessageList := openAIMessages(messages)
	finalMessage, errorValue := json.Marshal(openAIMessageList[len(openAIMessageList)-1])
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	start := bytes.LastIndex(document, finalMessage)
	if start <= 0 {
		t.Fatalf("the final message is not in the request as it was serialized: %s", document)
	}
	return document[:start]
}

// A prompt cache is a prefix match: the first byte that differs between two
// turns of one run throws away everything the provider had cached after it. So
// the turns of a run may differ only in what comes last.
func TestConsecutiveTurnsOfOneRunShareTheirPromptPrefix(t *testing.T) {
	backend := OpenRouterBackend{ProviderSort: "throughput"}
	schema := agentTurnSchema(t)
	firstTurn := turnMessages("you are the office agent", "how many were late in March?")
	secondTurn := turnMessages("you are the office agent", "and in April?")
	shapes := map[string]func(*testing.T, OpenRouterBackend, StructuredOutputSchema, []Message) []byte{
		"native action": actionTurnDocument,
		"structured":    structuredTurnDocument,
	}
	for name, documentOfTurn := range shapes {
		firstPrefix := prefixBeforeFinalMessage(t, documentOfTurn(t, backend, schema, firstTurn), firstTurn)
		secondPrefix := prefixBeforeFinalMessage(t, documentOfTurn(t, backend, schema, secondTurn), secondTurn)
		if !bytes.Equal(firstPrefix, secondPrefix) {
			t.Errorf("%s changed before its last message:\nfirst:  %s\nsecond: %s", name, firstPrefix, secondPrefix)
		}
		if !bytes.Contains(firstPrefix, []byte("you are the office agent")) {
			t.Errorf("%s left the system instruction out of the shared prefix: %s", name, firstPrefix)
		}
	}
}

func TestTheToolDefinitionsGoOutInTheSchemasOwnOrder(t *testing.T) {
	backend := OpenRouterBackend{ProviderSort: "throughput"}
	schema := agentTurnSchema(t)
	firstTurn := turnMessages("you are the office agent", "how many were late in March?")
	secondTurn := turnMessages("you are the office agent", "and in April?")

	firstTools := toolsOfDocument(t, actionTurnDocument(t, backend, schema, firstTurn))
	secondTools := toolsOfDocument(t, actionTurnDocument(t, backend, schema, secondTurn))
	if !bytes.Equal(firstTools, secondTools) {
		t.Fatalf("the tool definitions moved between turns:\nfirst:  %s\nsecond: %s", firstTools, secondTools)
	}
	expectedOrder := []string{
		nativeActionFunctionName("continue", "write"),
		nativeActionFunctionName("continue", "attendance_summary"),
		nativeActionFunctionName("continue", "platform_message"),
	}
	if functionNames := toolFunctionNames(t, firstTools); !stringSlicesEqual(functionNames, expectedOrder) {
		t.Fatalf("the tools did not follow the schema's own order: %v", functionNames)
	}
}

func toolsOfDocument(t *testing.T, document []byte) []byte {
	t.Helper()
	var request struct {
		Tools json.RawMessage `json:"tools"`
	}
	if errorValue := json.Unmarshal(document, &request); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(request.Tools) == 0 {
		t.Fatalf("the request carried no tools: %s", document)
	}
	return request.Tools
}

func toolFunctionNames(t *testing.T, tools []byte) []string {
	t.Helper()
	var definitions []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if errorValue := json.Unmarshal(tools, &definitions); errorValue != nil {
		t.Fatal(errorValue)
	}
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Function.Name)
	}
	return names
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
