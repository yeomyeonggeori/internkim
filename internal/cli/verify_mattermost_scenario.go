package cli

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

type mattermostScenario struct {
	Name                      string                             `json:"name"`
	RequiresSDKD              bool                               `json:"requiresSDKD"`
	SkillDirectoryPaths       []string                           `json:"skillDirectoryPaths"`
	AllowedTools              []string                           `json:"allowedTools"`
	CapabilityToolNames       []string                           `json:"capabilityToolNames"`
	CapabilityToolDescriptors []mattermostScenarioToolDescriptor `json:"capabilityToolDescriptors"`
	InitialToolNames          []string                           `json:"initialToolNames"`
	Steps                     []mattermostScenarioStep           `json:"steps"`
}

type mattermostScenarioToolDescriptor struct {
	Name             string `json:"name"`
	RequiresApproval bool   `json:"requiresApproval"`
}

type mattermostScenarioApprovalAction string

const mattermostScenarioApprovalApprove mattermostScenarioApprovalAction = "approve"

type mattermostScenarioStep struct {
	Prompt                  string                            `json:"prompt"`
	ExpectedToolCalls       []string                          `json:"expectedToolCalls"`
	ExpectedAnyToolCalls    []string                          `json:"expectedAnyToolCalls"`
	ExpectedEvents          []string                          `json:"expectedEvents"`
	ExpectedToolCallCounts  map[string]int                    `json:"expectedToolCallCounts"`
	ExpectedEventCounts     []mattermostScenarioEventCount    `json:"expectedEventCounts"`
	ExpectedAttachments     []string                          `json:"expectedAttachments"`
	ExpectedDocumentText    []string                          `json:"expectedDocumentText"`
	ExpectedWorkspaceFiles  []mattermostScenarioWorkspaceFile `json:"expectedWorkspaceFiles"`
	ForbiddenWorkspaceFiles []string                          `json:"forbiddenWorkspaceFiles"`
	ExpectedReplyFragments  []string                          `json:"expectedReplyFragments"`
	ForbiddenReplyFragments []string                          `json:"forbiddenReplyFragments"`
	MinimumReplyLength      int                               `json:"minimumReplyLength"`
	ExpectedTaskStatus      string                            `json:"expectedTaskStatus"`
	RequiresPublicURL       bool                              `json:"requiresPublicURL"`
	ExpectedPublicText      []string                          `json:"expectedPublicText"`
	ExpectedPublicControls  []string                          `json:"expectedPublicControls"`
	ApprovalAction          mattermostScenarioApprovalAction  `json:"approvalAction"`
}

type mattermostScenarioEventCount struct {
	Name           string `json:"name"`
	BodyFragment   string `json:"bodyFragment"`
	OutputFragment string `json:"outputFragment"`
	Count          int    `json:"count"`
}

type mattermostScenarioWorkspaceFile struct {
	PathGlob           string         `json:"pathGlob"`
	ContainsFragments  []string       `json:"containsFragments"`
	ForbiddenFragments []string       `json:"forbiddenFragments"`
	FragmentCounts     map[string]int `json:"fragmentCounts"`
}

type mattermostScenarioTaskDetail struct {
	TaskRun    mattermostScenarioTaskRun     `json:"taskRun"`
	TaskEvents []mattermostScenarioTaskEvent `json:"taskEvents"`
}

type mattermostScenarioTaskRun struct {
	TaskRunID string `json:"taskRunID"`
	Status    string `json:"status"`
}

type mattermostScenarioTaskEvent struct {
	TaskEventID string `json:"taskEventID"`
	Name        string `json:"name"`
	Body        string `json:"body"`
}

type mattermostScenarioPost struct {
	ID        string   `json:"id"`
	RootID    string   `json:"rootID"`
	UserID    string   `json:"userID"`
	Message   string   `json:"message"`
	FileIDs   []string `json:"fileIDs"`
	CreatedAt int64    `json:"createdAt"`
}

type mattermostScenarioStepResult struct {
	Prompt         string                              `json:"prompt"`
	TaskRunID      string                              `json:"taskRunID"`
	TaskStatus     string                              `json:"taskStatus"`
	BotMessage     string                              `json:"botMessage"`
	TaskEvents     []mattermostScenarioTaskEvent       `json:"taskEvents"`
	Attachments    []downloadedMattermostFile          `json:"attachments"`
	WorkspaceFiles []mattermostScenarioWorkspaceResult `json:"workspaceFiles"`
	PublicURL      string                              `json:"publicURL,omitempty"`
	LLMCallCount   int                                 `json:"llmCallCount"`
	AgentStepCount int                                 `json:"agentStepCount"`
	ToolCallCount  int                                 `json:"toolCallCount"`
	ProcessingMS   int64                               `json:"processingMs"`
}

type mattermostScenarioWorkspaceResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type mattermostScenarioEfficiencyObservation struct {
	StepIndex int    `json:"stepIndex"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Expected  int    `json:"expected"`
	Observed  int    `json:"observed"`
}

type mattermostScenarioResult struct {
	ScenarioName           string                                    `json:"scenarioName"`
	ChannelID              string                                    `json:"channelID"`
	ConversationID         string                                    `json:"conversationID"`
	UserID                 string                                    `json:"userID"`
	Posts                  []mattermostScenarioPost                  `json:"posts"`
	Steps                  []mattermostScenarioStepResult            `json:"steps"`
	TurnCount              int                                       `json:"turnCount"`
	ScenarioWallDurationMS int64                                     `json:"scenarioWallDurationMs"`
	EfficiencyObservations []mattermostScenarioEfficiencyObservation `json:"efficiencyObservations,omitempty"`
}

func (detail *mattermostScenarioTaskDetail) UnmarshalJSON(document []byte) error {
	type taskDetail mattermostScenarioTaskDetail
	trimmedDocument := bytes.TrimSpace(document)
	if len(trimmedDocument) == 0 {
		return errors.New("Mattermost task detail is empty")
	}
	if trimmedDocument[0] != '[' {
		return json.Unmarshal(trimmedDocument, (*taskDetail)(detail))
	}
	var details []taskDetail
	if errorValue := json.Unmarshal(trimmedDocument, &details); errorValue != nil {
		return errorValue
	}
	if len(details) == 0 {
		return errors.New("Mattermost task detail array is empty")
	}
	*detail = mattermostScenarioTaskDetail(details[0])
	return nil
}

func loadMattermostScenario(filePath string) (mattermostScenario, error) {
	document, errorValue := os.ReadFile(filePath)
	if errorValue != nil {
		return mattermostScenario{}, fmt.Errorf("read Mattermost scenario: %w", errorValue)
	}
	var scenario mattermostScenario
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&scenario); errorValue != nil {
		return mattermostScenario{}, fmt.Errorf("parse Mattermost scenario: %w", errorValue)
	}
	if errorValue := requireJSONEnd(decoder); errorValue != nil {
		return mattermostScenario{}, errorValue
	}
	if errorValue := validateMattermostScenario(scenario); errorValue != nil {
		return mattermostScenario{}, errorValue
	}
	return scenario, nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var trailingDocument json.RawMessage
	if errorValue := decoder.Decode(&trailingDocument); errorValue == io.EOF {
		return nil
	} else if errorValue != nil {
		return fmt.Errorf("parse Mattermost scenario trailing data: %w", errorValue)
	}
	return errors.New("Mattermost scenario has trailing data")
}

func validateMattermostScenario(scenario mattermostScenario) error {
	if strings.TrimSpace(scenario.Name) == "" {
		return errors.New("Mattermost scenario name is required")
	}
	if len(scenario.Steps) == 0 {
		return errors.New("Mattermost scenario steps are required")
	}
	for stepIndex, step := range scenario.Steps {
		if errorValue := validateMattermostScenarioBoundary(stepIndex, step); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateMattermostScenarioBoundary(stepIndex int, step mattermostScenarioStep) error {
	if strings.TrimSpace(step.Prompt) == "" {
		return fmt.Errorf("Mattermost scenario step %d prompt is required", stepIndex)
	}
	if step.MinimumReplyLength < 0 {
		return fmt.Errorf("Mattermost scenario step %d minimumReplyLength cannot be negative", stepIndex)
	}
	if step.ApprovalAction != "" && step.ApprovalAction != mattermostScenarioApprovalApprove {
		return fmt.Errorf("Mattermost scenario step %d approvalAction %q is unsupported", stepIndex, step.ApprovalAction)
	}
	for toolName, count := range step.ExpectedToolCallCounts {
		if strings.TrimSpace(toolName) == "" || count < 0 {
			return fmt.Errorf("Mattermost scenario step %d has invalid tool call count", stepIndex)
		}
	}
	for eventIndex, eventCount := range step.ExpectedEventCounts {
		if strings.TrimSpace(eventCount.Name) == "" || eventCount.Count < 0 {
			return fmt.Errorf("Mattermost scenario step %d event count %d is invalid", stepIndex, eventIndex)
		}
		if eventCount.OutputFragment != "" && eventCount.Name != "tool.capability.invoke.result" {
			return fmt.Errorf("Mattermost scenario step %d event count %d outputFragment requires tool.capability.invoke.result", stepIndex, eventIndex)
		}
		if eventCount.BodyFragment != "" && eventCount.OutputFragment != "" {
			return fmt.Errorf("Mattermost scenario step %d event count %d cannot combine bodyFragment and outputFragment", stepIndex, eventIndex)
		}
	}
	return nil
}

func validateMattermostScenarioResult(scenario mattermostScenario, result *mattermostScenarioResult) error {
	if result.ScenarioName != scenario.Name {
		return fmt.Errorf("Mattermost scenario result name %q does not match %q", result.ScenarioName, scenario.Name)
	}
	if len(result.Steps) != len(scenario.Steps) {
		return fmt.Errorf("Mattermost scenario returned %d steps, expected %d", len(result.Steps), len(scenario.Steps))
	}
	for stepIndex, expectedStep := range scenario.Steps {
		if scenario.RequiresSDKD {
			if errorValue := validateMattermostScenarioSDKD(stepIndex, result.Steps[stepIndex].TaskEvents); errorValue != nil {
				return errorValue
			}
		}
		if errorValue := validateMattermostScenarioStep(stepIndex, expectedStep, result.Steps[stepIndex], result); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateMattermostScenarioStep(stepIndex int, expected mattermostScenarioStep, actual mattermostScenarioStepResult, result *mattermostScenarioResult) error {
	if actual.Prompt != expected.Prompt {
		return fmt.Errorf("Mattermost scenario step %d prompt does not match", stepIndex)
	}
	if expected.ExpectedTaskStatus != "" && actual.TaskStatus != expected.ExpectedTaskStatus {
		return fmt.Errorf("Mattermost scenario step %d status %q does not match %q", stepIndex, actual.TaskStatus, expected.ExpectedTaskStatus)
	}
	if errorValue := validateMattermostScenarioReply(stepIndex, expected, actual.BotMessage); errorValue != nil {
		return errorValue
	}
	for _, attachmentName := range expected.ExpectedAttachments {
		if !hasMattermostScenarioAttachment(actual.Attachments, attachmentName) {
			return fmt.Errorf("Mattermost scenario step %d is missing attachment %q", stepIndex, attachmentName)
		}
	}
	if errorValue := validateMattermostScenarioDocuments(stepIndex, expected, actual.Attachments); errorValue != nil {
		return errorValue
	}
	if expected.RequiresPublicURL && strings.TrimSpace(actual.PublicURL) == "" {
		return fmt.Errorf("Mattermost scenario step %d is missing a published site URL", stepIndex)
	}
	if expected.RequiresPublicURL && !strings.Contains(actual.BotMessage, actual.PublicURL) {
		return fmt.Errorf("Mattermost scenario step %d reply does not expose its published site URL", stepIndex)
	}
	if errorValue := validateMattermostScenarioWorkspace(stepIndex, expected, actual.WorkspaceFiles); errorValue != nil {
		return errorValue
	}
	return validateMattermostScenarioEvents(stepIndex, expected, actual.TaskEvents, result)
}

func validateMattermostScenarioDocuments(stepIndex int, expected mattermostScenarioStep, files []downloadedMattermostFile) error {
	if len(expected.ExpectedDocumentText) == 0 {
		return nil
	}
	for _, file := range files {
		if !strings.HasSuffix(strings.ToLower(file.Filename), ".docx") {
			continue
		}
		document, errorValue := base64.StdEncoding.DecodeString(file.ContentBase64)
		if errorValue != nil {
			return fmt.Errorf("Mattermost scenario step %d decode DOCX: %w", stepIndex, errorValue)
		}
		if errorValue := validateDOCXDocument(document, expected.ExpectedDocumentText); errorValue != nil {
			return fmt.Errorf("Mattermost scenario step %d: %w", stepIndex, errorValue)
		}
		return nil
	}
	return fmt.Errorf("Mattermost scenario step %d has no DOCX attachment to inspect", stepIndex)
}

func validateDOCXDocument(document []byte, expectedText []string) error {
	reader, errorValue := zip.NewReader(bytes.NewReader(document), int64(len(document)))
	if errorValue != nil {
		return fmt.Errorf("open DOCX package: %w", errorValue)
	}
	entries := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		entries[file.Name] = file
	}
	for _, requiredName := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml"} {
		if entries[requiredName] == nil {
			return fmt.Errorf("DOCX package is missing %s", requiredName)
		}
	}
	content, errorValue := readDOCXEntry(entries["word/document.xml"])
	if errorValue != nil {
		return errorValue
	}
	if errorValue := validateDOCXXML(content); errorValue != nil {
		return errorValue
	}
	for _, fragment := range expectedText {
		if !bytes.Contains(content, []byte(fragment)) {
			return fmt.Errorf("DOCX document is missing %q", fragment)
		}
	}
	return nil
}

func readDOCXEntry(file *zip.File) ([]byte, error) {
	reader, errorValue := file.Open()
	if errorValue != nil {
		return nil, errorValue
	}
	defer reader.Close()
	content, errorValue := io.ReadAll(io.LimitReader(reader, 8<<20))
	if errorValue != nil {
		return nil, errorValue
	}
	return content, nil
}

func validateDOCXXML(document []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(document))
	for {
		if _, errorValue := decoder.Token(); errorValue == io.EOF {
			return nil
		} else if errorValue != nil {
			return fmt.Errorf("parse word/document.xml: %w", errorValue)
		}
	}
}

func validateMattermostScenarioReply(stepIndex int, expected mattermostScenarioStep, reply string) error {
	if expected.MinimumReplyLength > len([]rune(reply)) {
		return fmt.Errorf("Mattermost scenario step %d reply is too short", stepIndex)
	}
	for _, fragment := range expected.ExpectedReplyFragments {
		if !strings.Contains(reply, fragment) {
			return fmt.Errorf("Mattermost scenario step %d reply is missing %q", stepIndex, fragment)
		}
	}
	for _, fragment := range expected.ForbiddenReplyFragments {
		if strings.Contains(reply, fragment) {
			return fmt.Errorf("Mattermost scenario step %d reply contains forbidden fragment %q", stepIndex, fragment)
		}
	}
	return nil
}

func validateMattermostScenarioWorkspace(stepIndex int, expected mattermostScenarioStep, files []mattermostScenarioWorkspaceResult) error {
	for _, expectation := range expected.ExpectedWorkspaceFiles {
		matches := matchingMattermostScenarioWorkspaceFiles(files, expectation.PathGlob)
		if len(matches) == 0 {
			return fmt.Errorf("Mattermost scenario step %d is missing workspace file %q", stepIndex, expectation.PathGlob)
		}
		if len(matches) != 1 {
			return fmt.Errorf("Mattermost scenario step %d workspace glob %q matched %d files", stepIndex, expectation.PathGlob, len(matches))
		}
		if errorValue := validateMattermostScenarioWorkspaceFile(matches[0], expectation); errorValue != nil {
			return fmt.Errorf("Mattermost scenario step %d: %w", stepIndex, errorValue)
		}
	}
	for _, pathGlob := range expected.ForbiddenWorkspaceFiles {
		if len(matchingMattermostScenarioWorkspaceFiles(files, pathGlob)) > 0 {
			return fmt.Errorf("Mattermost scenario step %d contains forbidden workspace file %q", stepIndex, pathGlob)
		}
	}
	return nil
}

func validateMattermostScenarioWorkspaceFile(file mattermostScenarioWorkspaceResult, expectation mattermostScenarioWorkspaceFile) error {
	for _, fragment := range expectation.ContainsFragments {
		if !strings.Contains(file.Content, fragment) {
			return fmt.Errorf("workspace file %q is missing %q", file.Path, fragment)
		}
	}
	for _, fragment := range expectation.ForbiddenFragments {
		if strings.Contains(file.Content, fragment) {
			return fmt.Errorf("workspace file %q contains forbidden %q", file.Path, fragment)
		}
	}
	for fragment, expectedCount := range expectation.FragmentCounts {
		if strings.Count(file.Content, fragment) != expectedCount {
			return fmt.Errorf("workspace file %q count for %q is incorrect", file.Path, fragment)
		}
	}
	return nil
}

func matchingMattermostScenarioWorkspaceFiles(files []mattermostScenarioWorkspaceResult, pathGlob string) []mattermostScenarioWorkspaceResult {
	matches := []mattermostScenarioWorkspaceResult{}
	for _, file := range files {
		matched, errorValue := path.Match(pathGlob, strings.TrimPrefix(file.Path, "/workspace/"))
		if errorValue == nil && matched {
			matches = append(matches, file)
		}
	}
	sort.Slice(matches, func(firstIndex int, secondIndex int) bool {
		return matches[firstIndex].Path < matches[secondIndex].Path
	})
	return matches
}

func validateMattermostScenarioEvents(stepIndex int, expected mattermostScenarioStep, events []mattermostScenarioTaskEvent, result *mattermostScenarioResult) error {
	for _, eventName := range expected.ExpectedEvents {
		if countMattermostScenarioEvents(events, eventName, "") == 0 {
			return fmt.Errorf("Mattermost scenario step %d is missing event %q", stepIndex, eventName)
		}
	}
	for _, toolName := range expected.ExpectedToolCalls {
		if countMattermostScenarioToolEvents(events, toolName) == 0 {
			return fmt.Errorf("Mattermost scenario step %d is missing tool event %q", stepIndex, toolName)
		}
	}
	if len(expected.ExpectedAnyToolCalls) > 0 && !hasAnyMattermostScenarioToolEvent(events, expected.ExpectedAnyToolCalls) {
		return fmt.Errorf("Mattermost scenario step %d is missing any expected tool event", stepIndex)
	}
	for toolName, expectedCount := range expected.ExpectedToolCallCounts {
		observedCount := countMattermostScenarioToolEvents(events, toolName)
		if errorValue := recordMattermostScenarioCount(result, stepIndex, "tool", toolName, expectedCount, observedCount); errorValue != nil {
			return errorValue
		}
	}
	for _, eventCount := range expected.ExpectedEventCounts {
		observedCount := countMattermostScenarioExpectedEvents(events, eventCount)
		if errorValue := recordMattermostScenarioCount(result, stepIndex, "event", eventCount.Name, eventCount.Count, observedCount); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateMattermostScenarioSDKD(stepIndex int, events []mattermostScenarioTaskEvent) error {
	hasAuthoritativeCall := false
	hasSuccessfulCall := false
	for _, event := range events {
		if event.Name != "llm.call" {
			continue
		}
		var call struct {
			Kind         string `json:"kind"`
			SchemaName   string `json:"schemaName"`
			Transport    string `json:"transport"`
			Model        string `json:"model"`
			UsedFallback bool   `json:"usedFallback"`
			IsError      bool   `json:"isError"`
		}
		if json.Unmarshal([]byte(event.Body), &call) != nil || !isAuthoritativeSDKDCall(call.Kind, call.SchemaName) {
			continue
		}
		hasAuthoritativeCall = true
		if call.Transport != "sdkd" {
			return fmt.Errorf("Mattermost scenario step %d used %s transport for authoritative AI SDK call %s", stepIndex, call.Transport, call.SchemaName)
		}
		if call.UsedFallback {
			return fmt.Errorf("Mattermost scenario step %d used legacy fallback for authoritative AI SDK call %s", stepIndex, call.SchemaName)
		}
		if !call.IsError && strings.TrimSpace(call.Model) == "" {
			return fmt.Errorf("Mattermost scenario step %d has no model provenance for authoritative AI SDK call", stepIndex)
		}
		if !call.IsError {
			hasSuccessfulCall = true
		}
	}
	if !hasAuthoritativeCall {
		return fmt.Errorf("Mattermost scenario step %d has no authoritative AI SDK call evidence", stepIndex)
	}
	if !hasSuccessfulCall {
		return fmt.Errorf("Mattermost scenario step %d has no successful authoritative AI SDK call", stepIndex)
	}
	return nil
}

func isAuthoritativeSDKDCall(kind string, schemaName string) bool {
	if kind == "chat" || kind == "recovery_chat" || kind == "local_recovery_chat" {
		return true
	}
	return schemaName == "blueclaw_agent_turn_action" || schemaName == "blueclaw_turn_router"
}

func hasAnyMattermostScenarioToolEvent(events []mattermostScenarioTaskEvent, toolNames []string) bool {
	for _, toolName := range toolNames {
		if countMattermostScenarioToolEvents(events, toolName) > 0 {
			return true
		}
	}
	return false
}

func recordMattermostScenarioCount(result *mattermostScenarioResult, stepIndex int, kind string, name string, expected int, observed int) error {
	if expected == 0 && observed != 0 {
		return fmt.Errorf("Mattermost scenario step %d has forbidden %s %q count %d", stepIndex, kind, name, observed)
	}
	if expected > 0 && observed == 0 {
		return fmt.Errorf("Mattermost scenario step %d is missing %s %q", stepIndex, kind, name)
	}
	if expected > 0 && observed != expected {
		observation := mattermostScenarioEfficiencyObservation{
			StepIndex: stepIndex,
			Kind:      kind,
			Name:      name,
			Expected:  expected,
			Observed:  observed,
		}
		for observationIndex := range result.EfficiencyObservations {
			existing := result.EfficiencyObservations[observationIndex]
			if existing.StepIndex == stepIndex && existing.Kind == kind && existing.Name == name {
				result.EfficiencyObservations[observationIndex] = observation
				return nil
			}
		}
		result.EfficiencyObservations = append(result.EfficiencyObservations, observation)
	}
	return nil
}

func hasMattermostScenarioAttachment(files []downloadedMattermostFile, expectedName string) bool {
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file.Filename), strings.ToLower(expectedName)) {
			return true
		}
	}
	return false
}

func countMattermostScenarioEvents(events []mattermostScenarioTaskEvent, expectedName string, bodyFragment string) int {
	count := 0
	for _, event := range events {
		if event.Name == expectedName && strings.Contains(event.Body, bodyFragment) {
			count++
		}
	}
	return count
}

func countMattermostScenarioExpectedEvents(events []mattermostScenarioTaskEvent, expected mattermostScenarioEventCount) int {
	if expected.OutputFragment == "" {
		return countMattermostScenarioEvents(events, expected.Name, expected.BodyFragment)
	}
	count := 0
	for _, event := range events {
		if event.Name == expected.Name && capabilityResultOutputContains(event.Body, expected.OutputFragment) {
			count++
		}
	}
	return count
}

func capabilityResultOutputContains(body string, fragment string) bool {
	var result struct {
		Output json.RawMessage `json:"output"`
	}
	if json.Unmarshal([]byte(body), &result) != nil {
		return false
	}
	return bytes.Contains(result.Output, []byte(fragment))
}

func countMattermostScenarioToolEvents(events []mattermostScenarioTaskEvent, toolName string) int {
	count := 0
	for _, event := range events {
		if event.Name == "tool."+toolName+".requested" || isRequestedCapabilityOperation(event, toolName) {
			count++
		}
	}
	return count
}

func isRequestedCapabilityOperation(event mattermostScenarioTaskEvent, operation string) bool {
	if event.Name != "tool.capability.invoke.requested" {
		return false
	}
	var body struct {
		Operation string `json:"operation"`
		Input     struct {
			Operation string `json:"operation"`
		} `json:"input"`
	}
	if json.Unmarshal([]byte(event.Body), &body) != nil {
		return false
	}
	return body.Operation == operation || body.Input.Operation == operation
}
