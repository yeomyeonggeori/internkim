package llmbackend

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type toolArgumentContractCase struct {
	Name          string
	Descriptor    capabilities.Descriptor
	Prompt        string
	ExpectedTool  string
	RequiredField string
}

func toolArgumentContractCases(t *testing.T) []toolArgumentContractCase {
	t.Helper()
	webDescriptors := capabilities.WebDescriptors()
	flowDescriptors := capabilities.FlowDescriptors()
	return []toolArgumentContractCase{
		{
			Name:          "web.search.query",
			Descriptor:    findLiveDescriptor(t, webDescriptors, "web.search"),
			Prompt:        "Search the web for the official release date of the next SpaceX Starship flight.",
			ExpectedTool:  "web.search",
			RequiredField: "query",
		},
		{
			Name:          "task.add.prompt",
			Descriptor:    findLiveDescriptor(t, flowDescriptors, "task.add"),
			Prompt:        "Add a new work task: 분기 보고서 초안 작성.",
			ExpectedTool:  "task.add",
			RequiredField: "prompt",
		},
	}
}

func toolContractModelsFromEnv() []string {
	configured := strings.TrimSpace(testEnvValue("OPENROUTER_TOOL_MODELS", ""))
	if configured != "" {
		models := []string{}
		for _, model := range strings.Split(configured, ",") {
			if trimmed := strings.TrimSpace(model); trimmed != "" {
				models = append(models, trimmed)
			}
		}
		return models
	}
	return []string{
		"google/gemini-3.5-flash",
		"x-ai/grok-4.3",
		"google/gemini-3.1-flash-lite",
		"google/gemini-3.1-flash-lite",
		"z-ai/glm-5.2",
	}
}

func TestOpenRouterLiveNativeToolArgumentsAreFilledFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	for _, modelName := range toolContractModelsFromEnv() {
		for _, contractCase := range toolArgumentContractCases(t) {
			t.Run(modelName+"/"+contractCase.Name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				request := StructuredRequest{
					Model:    modelName,
					Messages: []Message{{Role: "user", Content: contractCase.Prompt}},
					StructuredOutputSchema: StructuredOutputSchema{
						Name:               "blueclaw_agent_turn_action",
						Document:           testActionSchemaForDescriptors(t, []capabilities.Descriptor{contractCase.Descriptor}),
						IsStrictlyEnforced: true,
					},
				}
				response, errorValue := backend.CompleteStructured(ctx, request)
				if errorValue != nil {
					t.Fatalf("live native tool call failed: %v", errorValue)
				}
				if response.ConstraintMode != ConstraintModeNativeToolCall {
					t.Fatalf("expected native_tool_call constraint mode, got %q (content=%s)", response.ConstraintMode, response.Content)
				}
				assertToolArgumentFilled(t, response.Content, contractCase)
			})
		}
	}
}

func TestOpenRouterLiveNativeEitherOrToolFilledInLargeToolSetFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	descriptors := []capabilities.Descriptor{}
	descriptors = append(descriptors, capabilities.FileDescriptors()...)
	descriptors = append(descriptors, capabilities.FlowDescriptors()...)
	descriptors = append(descriptors, capabilities.CalendarDescriptors()...)
	descriptors = append(descriptors, capabilities.WebDescriptors()...)
	schemaDocument := testActionSchemaForDescriptors(t, descriptors)

	for _, modelName := range toolContractModelsFromEnv() {
		t.Run(modelName, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			request := StructuredRequest{
				Model:    modelName,
				Messages: []Message{{Role: "user", Content: "Read the file stored at home/notes/launch-plan.md and summarize what it contains."}},
				StructuredOutputSchema: StructuredOutputSchema{
					Name:               "blueclaw_agent_turn_action",
					Document:           schemaDocument,
					IsStrictlyEnforced: true,
				},
			}
			response, errorValue := backend.CompleteStructured(ctx, request)
			if errorValue != nil {
				t.Fatalf("live native tool call failed: %v", errorValue)
			}
			if response.ConstraintMode != ConstraintModeNativeToolCall {
				t.Fatalf("expected native_tool_call mode, got %q (content=%s)", response.ConstraintMode, response.Content)
			}
			assertActionToolInputNotEmpty(t, response.Content)
		})
	}
}

func TestOpenRouterLiveFullDeviceToolSetFitsAndFillsFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	descriptors := capabilities.DeviceDescriptors()
	schemaDocument := testActionSchemaForDescriptors(t, descriptors)

	for _, modelName := range toolContractModelsFromEnv() {
		t.Run(modelName, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			request := StructuredRequest{
				Model:    modelName,
				Messages: []Message{{Role: "user", Content: "Read the file stored at home/notes/launch-plan.md and summarize what it contains."}},
				StructuredOutputSchema: StructuredOutputSchema{
					Name:               "blueclaw_agent_turn_action",
					Document:           schemaDocument,
					IsStrictlyEnforced: true,
				},
			}
			response, errorValue := backend.CompleteStructured(ctx, request)
			if errorValue != nil {
				t.Fatalf("full device tool set (%d tools) was rejected or failed: %v", len(descriptors), errorValue)
			}
			if response.ConstraintMode != ConstraintModeNativeToolCall {
				t.Fatalf("expected native_tool_call mode, got %q (content=%s)", response.ConstraintMode, response.Content)
			}
			assertActionToolInputNotEmpty(t, response.Content)
		})
	}
}

func assertActionToolInputNotEmpty(t *testing.T, content string) {
	t.Helper()
	var action struct {
		ToolName  string                     `json:"toolName"`
		ToolInput map[string]json.RawMessage `json:"toolInput"`
	}
	if errorValue := json.Unmarshal([]byte(content), &action); errorValue != nil {
		t.Fatalf("could not parse action content %q: %v", content, errorValue)
	}
	hasNonNullValue := false
	for _, rawValue := range action.ToolInput {
		trimmed := strings.TrimSpace(string(rawValue))
		if trimmed != "" && trimmed != "null" && trimmed != "\"\"" {
			hasNonNullValue = true
			break
		}
	}
	if !hasNonNullValue {
		t.Fatalf("tool %q received empty/all-null arguments in a large tool set (content=%s)", action.ToolName, content)
	}
}

func assertToolArgumentFilled(t *testing.T, content string, contractCase toolArgumentContractCase) {
	t.Helper()
	var action struct {
		ToolName  string                     `json:"toolName"`
		ToolInput map[string]json.RawMessage `json:"toolInput"`
	}
	if errorValue := json.Unmarshal([]byte(content), &action); errorValue != nil {
		t.Fatalf("could not parse action content %q: %v", content, errorValue)
	}
	if action.ToolName != contractCase.ExpectedTool {
		t.Fatalf("expected tool %q, got %q (content=%s)", contractCase.ExpectedTool, action.ToolName, content)
	}
	rawValue, isPresent := action.ToolInput[contractCase.RequiredField]
	if !isPresent {
		t.Fatalf("required field %q missing from toolInput (content=%s)", contractCase.RequiredField, content)
	}
	var stringValue string
	if json.Unmarshal(rawValue, &stringValue) == nil {
		if strings.TrimSpace(stringValue) == "" {
			t.Fatalf("required field %q was empty (content=%s)", contractCase.RequiredField, content)
		}
		return
	}
	if strings.TrimSpace(string(rawValue)) == "" || string(rawValue) == "null" || string(rawValue) == "\"\"" {
		t.Fatalf("required field %q was empty/null (content=%s)", contractCase.RequiredField, content)
	}
}
