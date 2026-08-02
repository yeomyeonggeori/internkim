package llmbackend

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func experimentModelName() string {
	return testEnvValue("OPENROUTER_STRIP_EXPERIMENT_MODEL", "google/gemini-3.1-flash-lite")
}

const experimentTrialsPerCase = 10
const experimentWorkerCount = 4

var experimentISOTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}`)

type experimentVariant struct {
	Name              string
	KeepRequiredDepth int
}

type experimentScenario struct {
	Name                  string
	Prompt                string
	ExpectedTool          string
	PrimaryDescriptor     capabilities.Descriptor
	RequiredFieldsToCheck []string
	IsUnderspecified      bool
}

type experimentTrialResult struct {
	Variant             string
	Scenario            string
	Trial               int
	TransportError      string
	SchemaRejection     bool
	ToolCallReturned    bool
	CorrectTool         bool
	RequiredComplete    bool
	Fabricated          bool
	LatencyMilliseconds int64
	ArgumentsJSON       string
}

func experimentVariants() []experimentVariant {
	return []experimentVariant{
		{Name: "strip-all", KeepRequiredDepth: 0},
		{Name: "keep-depth1", KeepRequiredDepth: 1},
		{Name: "keep-all", KeepRequiredDepth: 1 << 20},
	}
}

func experimentScenarios(t *testing.T) []experimentScenario {
	t.Helper()
	calendarAddDescriptor := findLiveDescriptor(t, capabilities.CalendarDescriptors(), "calendar_add")
	taskAddDescriptor := findLiveDescriptor(t, capabilities.FlowDescriptors(), "task_add")
	return []experimentScenario{
		{
			Name:                  "calendar-full",
			Prompt:                "내일 오후 3시부터 4시까지 팀 회의 일정 잡아줘. 오늘은 2026-07-02 수요일, 시간대 Asia/Seoul.",
			ExpectedTool:          "calendar_add",
			PrimaryDescriptor:     calendarAddDescriptor,
			RequiredFieldsToCheck: experimentDescriptorRequiredFields(t, calendarAddDescriptor),
		},
		{
			Name:                  "task-add",
			Prompt:                "분기 보고서 초안 작성 업무를 추가해줘.",
			ExpectedTool:          "task_add",
			PrimaryDescriptor:     taskAddDescriptor,
			RequiredFieldsToCheck: experimentDescriptorRequiredFields(t, taskAddDescriptor),
		},
		{
			Name:                  "underspecified",
			Prompt:                "회의 잡아줘.",
			ExpectedTool:          "calendar_add",
			PrimaryDescriptor:     calendarAddDescriptor,
			RequiredFieldsToCheck: experimentDescriptorRequiredFields(t, calendarAddDescriptor),
			IsUnderspecified:      true,
		},
	}
}

func experimentDescriptorRequiredFields(t *testing.T, descriptor capabilities.Descriptor) []string {
	t.Helper()
	var schema struct {
		Required []string `json:"required"`
	}
	if errorValue := json.Unmarshal(descriptor.InputSchema, &schema); errorValue != nil {
		t.Fatalf("descriptor %s input schema is invalid: %v", descriptor.Name, errorValue)
	}
	return schema.Required
}

func TestOpenRouterLiveNestedRequiredStripExperiment(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	backend.ModelName = experimentModelName()

	variants := experimentVariants()
	scenarios := experimentScenarios(t)
	webSearchDescriptor := findLiveDescriptor(t, capabilities.WebDescriptors(), "web_search")
	schemaByVariantScenario := experimentSchemaDocuments(t, variants, scenarios, webSearchDescriptor)

	type experimentJob struct {
		variant  experimentVariant
		scenario experimentScenario
		trial    int
	}
	jobs := make(chan experimentJob, len(variants)*len(scenarios)*experimentTrialsPerCase)
	for _, variant := range variants {
		for _, scenario := range scenarios {
			for trial := 1; trial <= experimentTrialsPerCase; trial++ {
				jobs <- experimentJob{variant: variant, scenario: scenario, trial: trial}
			}
		}
	}
	close(jobs)

	resultsChannel := make(chan experimentTrialResult, cap(jobs))
	var waitGroup sync.WaitGroup
	for workerIndex := 0; workerIndex < experimentWorkerCount; workerIndex++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			for job := range jobs {
				schemaDocument := schemaByVariantScenario[job.variant.Name][job.scenario.Name]
				resultsChannel <- runExperimentTrial(backend, schemaDocument, job.variant, job.scenario, job.trial)
			}
		}()
	}
	waitGroup.Wait()
	close(resultsChannel)

	results := make([]experimentTrialResult, 0, len(jobs))
	for result := range resultsChannel {
		results = append(results, result)
	}

	schemaSizes := experimentSchemaByteSizes(schemaByVariantScenario)
	t.Log(experimentReport(variants, scenarios, results, schemaSizes))
}

func experimentSchemaDocuments(t *testing.T, variants []experimentVariant, scenarios []experimentScenario, webSearchDescriptor capabilities.Descriptor) map[string]map[string]json.RawMessage {
	t.Helper()
	schemaByVariantScenario := map[string]map[string]json.RawMessage{}
	for _, variant := range variants {
		schemaByVariantScenario[variant.Name] = map[string]json.RawMessage{}
		for _, scenario := range scenarios {
			descriptors := []capabilities.Descriptor{scenario.PrimaryDescriptor, webSearchDescriptor}
			schemaByVariantScenario[variant.Name][scenario.Name] = experimentActionSchemaForDescriptors(t, descriptors, variant.KeepRequiredDepth)
		}
	}
	return schemaByVariantScenario
}

func runExperimentTrial(backend OpenRouterBackend, schemaDocument json.RawMessage, variant experimentVariant, scenario experimentScenario, trial int) experimentTrialResult {
	result := experimentTrialResult{
		Variant:  variant.Name,
		Scenario: scenario.Name,
		Trial:    trial,
	}

	ctx := context.Background()

	seed := int64(trial)
	temperature := 0.0
	request := StructuredRequest{
		Model:    experimentModelName(),
		Messages: []Message{{Role: "user", Content: scenario.Prompt}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           schemaDocument,
			IsStrictlyEnforced: true,
		},
		GenerationOptions: &GenerationOptions{Seed: &seed, Temperature: &temperature},
	}

	startTime := time.Now()
	content, errorValue := sendExperimentNativeActionRequest(ctx, backend, request)
	result.LatencyMilliseconds = time.Since(startTime).Milliseconds()

	if errorValue != nil {
		result.TransportError = errorValue.Error()
		result.SchemaRejection = experimentIsSchemaRejection(result.TransportError)
		return result
	}

	result.ToolCallReturned = true
	result.ArgumentsJSON = content

	var action struct {
		ToolName  string                     `json:"toolName"`
		ToolInput map[string]json.RawMessage `json:"toolInput"`
	}
	if errorValue := json.Unmarshal([]byte(content), &action); errorValue != nil {
		result.TransportError = "could not parse action content: " + errorValue.Error()
		return result
	}

	result.CorrectTool = action.ToolName == scenario.ExpectedTool
	result.RequiredComplete = experimentRequiredFieldsPresent(action.ToolInput, scenario.RequiredFieldsToCheck)
	if scenario.IsUnderspecified {
		result.Fabricated = experimentISOTimestampPattern.MatchString(content)
	}
	return result
}

func sendExperimentNativeActionRequest(ctx context.Context, backend OpenRouterBackend, request StructuredRequest) (string, error) {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return "", errorValue
	}
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(request.StructuredOutputSchema)
	if errorValue != nil {
		return "", errorValue
	}
	if !isActionSchema {
		return "", fmt.Errorf("experiment schema was not recognized as an action schema")
	}
	requestDocument, lintResult, errorValue := buildOpenRouterChatActionRequest(request, backend.resolveModelName(request.Model), toolSet.Tools, toolSet.NativeSchemaLint)
	if errorValue != nil {
		return "", errorValue
	}
	content, _, errorValue := backend.sendChatAction(ctx, apiKey, requestDocument, toolSet, lintResult)
	return content, errorValue
}

func experimentIsSchemaRejection(transportError string) bool {
	normalizedError := strings.ToUpper(transportError)
	return strings.Contains(normalizedError, "400") ||
		strings.Contains(normalizedError, "INVALID_ARGUMENT") ||
		strings.Contains(normalizedError, "SCHEMA")
}

func experimentRequiredFieldsPresent(toolInput map[string]json.RawMessage, requiredFieldsToCheck []string) bool {
	for _, fieldName := range requiredFieldsToCheck {
		rawValue, isPresent := toolInput[fieldName]
		if !isPresent || !experimentValueIsNonEmpty(rawValue) {
			return false
		}
	}
	return true
}

func experimentValueIsNonEmpty(rawValue json.RawMessage) bool {
	trimmedValue := strings.TrimSpace(string(rawValue))
	if trimmedValue == "" || trimmedValue == "null" || trimmedValue == `""` {
		return false
	}
	var stringValue string
	if json.Unmarshal(rawValue, &stringValue) == nil {
		return strings.TrimSpace(stringValue) != ""
	}
	return true
}

func experimentActionSchemaForDescriptors(t *testing.T, descriptors []capabilities.Descriptor, keepRequiredDepth int) json.RawMessage {
	t.Helper()
	variants := make([]any, 0, len(descriptors))
	for _, descriptor := range descriptors {
		inputSchema := descriptor.InputSchema
		if len(inputSchema) == 0 {
			inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var toolInput any
		if errorValue := json.Unmarshal(inputSchema, &toolInput); errorValue != nil {
			t.Fatalf("input schema for %s is invalid: %v", descriptor.Name, errorValue)
		}
		toolInput = experimentPortableNestedSchema(toolInput, 0, keepRequiredDepth)
		variants = append(variants, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":    map[string]any{"type": "string", "enum": []string{"continue"}},
				"toolName":  map[string]any{"type": "string", "enum": []string{descriptor.Name}},
				"toolInput": toolInput,
				"executionStateUpdate": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
				"nextStepPlan": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"objective":        map[string]any{"type": "string"},
						"expectedTools":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"doneCriteria":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"risk":             map[string]any{"type": "string"},
						"workingSetReason": map[string]any{"type": "string"},
					},
					"required": []string{"objective", "expectedTools", "doneCriteria", "risk", "workingSetReason"},
				},
			},
			"required": []string{"action", "toolName", "toolInput", "executionStateUpdate", "nextStepPlan"},
		})
	}
	document, errorValue := json.Marshal(map[string]any{"oneOf": variants})
	if errorValue != nil {
		t.Fatalf("expected action schema: %v", errorValue)
	}
	return document
}

func experimentPortableNestedSchema(value any, currentDepth int, keepRequiredDepth int) any {
	document, isObject := value.(map[string]any)
	if isObject {
		clone := map[string]any{}
		for fieldName, fieldValue := range document {
			switch {
			case fieldName == "required":
				if currentDepth < keepRequiredDepth {
					clone[fieldName] = fieldValue
				}
			case fieldName == "type" && fieldValue == "integer":
				clone[fieldName] = "number"
			case fieldName == "properties":
				clone[fieldName] = experimentPortableNestedSchemaProperties(fieldValue, currentDepth+1, keepRequiredDepth)
			default:
				clone[fieldName] = experimentPortableNestedSchema(fieldValue, currentDepth, keepRequiredDepth)
			}
		}
		if clone["type"] == "object" {
			if _, isFound := clone["properties"]; !isFound {
				clone["properties"] = map[string]any{}
			}
		}
		return clone
	}
	values, isArray := value.([]any)
	if isArray {
		clone := make([]any, 0, len(values))
		for _, item := range values {
			clone = append(clone, experimentPortableNestedSchema(item, currentDepth, keepRequiredDepth))
		}
		return clone
	}
	return value
}

func experimentPortableNestedSchemaProperties(value any, propertyDepth int, keepRequiredDepth int) any {
	document, isObject := value.(map[string]any)
	if !isObject {
		return value
	}
	clone := map[string]any{}
	for propertyName, propertySchema := range document {
		clone[propertyName] = experimentPortableNestedSchema(propertySchema, propertyDepth, keepRequiredDepth)
	}
	return clone
}

func experimentSchemaByteSizes(schemaByVariantScenario map[string]map[string]json.RawMessage) map[string]map[string]int {
	sizes := map[string]map[string]int{}
	for variantName, scenarioSchemas := range schemaByVariantScenario {
		sizes[variantName] = map[string]int{}
		for scenarioName, schemaDocument := range scenarioSchemas {
			sizes[variantName][scenarioName] = len(schemaDocument)
		}
	}
	return sizes
}

func experimentReport(variants []experimentVariant, scenarios []experimentScenario, results []experimentTrialResult, schemaSizes map[string]map[string]int) string {
	var builder strings.Builder
	builder.WriteString("\n| variant | scenario | calls | transportErrors | schemaRejections | toolCallReturned | correctTool | requiredComplete | fabricated |\n")
	builder.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, variant := range variants {
		for _, scenario := range scenarios {
			builder.WriteString(experimentAggregateRow(variant.Name, scenario.Name, results))
		}
	}

	builder.WriteString("\n| variant | scenario | schema bytes |\n|---|---|---|\n")
	for _, variant := range variants {
		for _, scenario := range scenarios {
			builder.WriteString(fmt.Sprintf("| %s | %s | %d |\n", variant.Name, scenario.Name, schemaSizes[variant.Name][scenario.Name]))
		}
	}

	builder.WriteString("\nDeduplicated transport errors:\n")
	for _, transportError := range experimentDeduplicatedTransportErrors(results) {
		builder.WriteString("- " + transportError + "\n")
	}

	builder.WriteString("\nUnderspecified raw arguments (first 2 trials per variant):\n")
	for _, variant := range variants {
		for _, result := range results {
			if result.Variant != variant.Name || result.Scenario != "underspecified" || result.Trial > 2 {
				continue
			}
			builder.WriteString(fmt.Sprintf("- %s trial %d: %s\n", variant.Name, result.Trial, result.ArgumentsJSON))
		}
	}

	return builder.String()
}

func experimentAggregateRow(variantName string, scenarioName string, results []experimentTrialResult) string {
	calls, transportErrors, schemaRejections, toolCallReturned, correctTool, requiredComplete, fabricated := 0, 0, 0, 0, 0, 0, 0
	for _, result := range results {
		if result.Variant != variantName || result.Scenario != scenarioName {
			continue
		}
		calls++
		if result.TransportError != "" {
			transportErrors++
		}
		if result.SchemaRejection {
			schemaRejections++
		}
		if result.ToolCallReturned {
			toolCallReturned++
		}
		if result.CorrectTool {
			correctTool++
		}
		if result.RequiredComplete {
			requiredComplete++
		}
		if result.Fabricated {
			fabricated++
		}
	}
	return fmt.Sprintf("| %s | %s | %d | %d | %d | %d | %d | %d | %d |\n",
		variantName, scenarioName, calls, transportErrors, schemaRejections, toolCallReturned, correctTool, requiredComplete, fabricated)
}

func experimentDeduplicatedTransportErrors(results []experimentTrialResult) []string {
	uniqueErrors := map[string]bool{}
	for _, result := range results {
		if result.TransportError != "" {
			uniqueErrors[result.TransportError] = true
		}
	}
	deduplicated := make([]string, 0, len(uniqueErrors))
	for transportError := range uniqueErrors {
		deduplicated = append(deduplicated, transportError)
	}
	sort.Strings(deduplicated)
	return deduplicated
}
