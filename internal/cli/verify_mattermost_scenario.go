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
	"time"
)

type mattermostScenario struct {
	Name                      string                             `json:"name"`
	RequiresLLMD              bool                               `json:"requiresLLMD"`
	ExpectedLLMProvider       string                             `json:"expectedLLMProvider"`
	SkillDirectoryPaths       []string                           `json:"skillDirectoryPaths"`
	AllowedTools              []string                           `json:"allowedTools"`
	CapabilityToolNames       []string                           `json:"capabilityToolNames"`
	CapabilityToolDescriptors []mattermostScenarioToolDescriptor `json:"capabilityToolDescriptors"`
	InitialToolNames          []string                           `json:"initialToolNames"`
	Steps                     []mattermostScenarioStep           `json:"steps"`
	MaximumModelTier          string                             `json:"-"`
}

type mattermostScenarioExposureEvidence struct {
	ExposedToolIDs       []string `json:"exposedToolIDs"`
	SelectedSkillToolIDs []string `json:"selectedSkillToolIDs"`
	SelectionSource      string   `json:"selectionSource"`
	UsedFallbackGroups   bool     `json:"usedFallbackGroups"`
}

type mattermostScenarioInstructionEvidence struct {
	ExposedToolNames            []string            `json:"exposedToolNames"`
	SelectedSkillToolReferences map[string][]string `json:"selectedSkillToolReferences"`
}

type mattermostScenarioWorkingSetEvidence struct {
	Exposure mattermostScenarioExposureEvidence `json:"exposure"`
}

type mattermostScenarioToolDescriptor struct {
	Name             string `json:"name"`
	RequiresApproval bool   `json:"requiresApproval"`
}

type mattermostScenarioApprovalAction string

const mattermostScenarioApprovalApprove mattermostScenarioApprovalAction = "approve"

type mattermostScenarioStep struct {
	Prompt                      string                            `json:"prompt"`
	ExpectedToolCalls           []string                          `json:"expectedToolCalls"`
	ExpectedAnyToolCalls        []string                          `json:"expectedAnyToolCalls"`
	ExpectedEvents              []string                          `json:"expectedEvents"`
	ExpectedToolCallCounts      map[string]int                    `json:"expectedToolCallCounts"`
	ExpectedExactToolCallCounts map[string]int                    `json:"expectedExactToolCallCounts"`
	ExpectedEventCounts         []mattermostScenarioEventCount    `json:"expectedEventCounts"`
	ExpectedAttachments         []string                          `json:"expectedAttachments"`
	ExpectedAttachmentText      []string                          `json:"expectedAttachmentText"`
	ExpectedDocumentText        []string                          `json:"expectedDocumentText"`
	ExpectedWorkspaceFiles      []mattermostScenarioWorkspaceFile `json:"expectedWorkspaceFiles"`
	ForbiddenWorkspaceFiles     []string                          `json:"forbiddenWorkspaceFiles"`
	ExpectedReplyFragments      []string                          `json:"expectedReplyFragments"`
	ForbiddenReplyFragments     []string                          `json:"forbiddenReplyFragments"`
	MinimumReplyLength          int                               `json:"minimumReplyLength"`
	ExpectedTaskStatus          string                            `json:"expectedTaskStatus"`
	RequiresPublicURL           bool                              `json:"requiresPublicURL"`
	ExpectedPublicText          []string                          `json:"expectedPublicText"`
	ExpectedPublicControls      []string                          `json:"expectedPublicControls"`
	ApprovalAction              mattermostScenarioApprovalAction  `json:"approvalAction"`
}

type mattermostScenarioEventCount struct {
	Name           string   `json:"name"`
	AnyOfNames     []string `json:"anyOfNames"`
	BodyFragment   string   `json:"bodyFragment"`
	OutputFragment string   `json:"outputFragment"`
	Count          int      `json:"count"`
	Exact          bool     `json:"exact"`
	Advisory       bool     `json:"advisory"`
}

func (expectation mattermostScenarioEventCount) eventNames() []string {
	if len(expectation.AnyOfNames) > 0 {
		return expectation.AnyOfNames
	}
	return []string{expectation.Name}
}

func (expectation mattermostScenarioEventCount) matchesEventName(name string) bool {
	for _, expectedName := range expectation.eventNames() {
		if name == expectedName {
			return true
		}
	}
	return false
}

func (expectation mattermostScenarioEventCount) eventNamesLabel() string {
	return strings.Join(expectation.eventNames(), "|")
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
	TaskEventID string    `json:"taskEventID"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"createdAt"`
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
	Prompt                 string                              `json:"prompt"`
	TaskRunID              string                              `json:"taskRunID"`
	TaskStatus             string                              `json:"taskStatus"`
	BotPostID              string                              `json:"botPostID"`
	BotMessage             string                              `json:"botMessage"`
	TaskEvents             []mattermostScenarioTaskEvent       `json:"taskEvents"`
	Attachments            []downloadedMattermostFile          `json:"attachments"`
	WorkspaceFiles         []mattermostScenarioWorkspaceResult `json:"workspaceFiles"`
	WorkspaceEvidenceError string                              `json:"workspaceEvidenceError,omitempty"`
	PublicURL              string                              `json:"publicURL,omitempty"`
	LLMCallCount           int                                 `json:"llmCallCount"`
	AgentStepCount         int                                 `json:"agentStepCount"`
	ToolCallCount          int                                 `json:"toolCallCount"`
	ProcessingMS           int64                               `json:"processingMs"`
	TokenUsage             mattermostScenarioTokenUsage        `json:"tokenUsage"`
}

type mattermostScenarioTokenUsage struct {
	LLMCallCount       int     `json:"llmCallCount"`
	PromptTokens       int64   `json:"promptTokens"`
	CompletionTokens   int64   `json:"completionTokens"`
	TotalTokens        int64   `json:"totalTokens"`
	CachedPromptTokens int64   `json:"cachedPromptTokens"`
	ReasoningTokens    int64   `json:"reasoningTokens"`
	CostUSD            float64 `json:"costUSD"`
	CacheHitRatio      float64 `json:"cacheHitRatio"`
}

type mattermostScenarioWorkspaceResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type mattermostScenarioRuntimeValues struct {
	NextFriday string
	Tomorrow   string
}

var mattermostScenarioTimeZone = time.FixedZone("Asia/Seoul", 9*60*60)

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
	AdvisoryFailures       []mattermostScenarioAdvisoryFailure       `json:"advisoryFailures,omitempty"`
	Attempt                int                                       `json:"attempt,omitempty"`
	TokenUsage             mattermostScenarioTokenUsage              `json:"tokenUsage"`
	TokensPerStep          float64                                   `json:"tokensPerStep"`
}

type mattermostScenarioAdvisoryFailure struct {
	StepIndex int    `json:"stepIndex"`
	Name      string `json:"name"`
	Reason    string `json:"reason"`
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
	resolveMattermostScenarioExpectedValues(&scenario, time.Now())
	if errorValue := validateMattermostScenario(scenario); errorValue != nil {
		return mattermostScenario{}, errorValue
	}
	return scenario, nil
}

func resolveMattermostScenarioExpectedValues(scenario *mattermostScenario, currentTime time.Time) {
	if scenario == nil {
		return
	}
	runtimeValues := mattermostScenarioValuesAt(currentTime)
	for stepIndex := range scenario.Steps {
		for eventIndex := range scenario.Steps[stepIndex].ExpectedEventCounts {
			eventCount := &scenario.Steps[stepIndex].ExpectedEventCounts[eventIndex]
			eventCount.BodyFragment = replaceMattermostScenarioRuntimeValues(eventCount.BodyFragment, runtimeValues)
			eventCount.OutputFragment = replaceMattermostScenarioRuntimeValues(eventCount.OutputFragment, runtimeValues)
		}
	}
}

func mattermostScenarioValuesAt(currentTime time.Time) mattermostScenarioRuntimeValues {
	localTime := currentTime.In(mattermostScenarioTimeZone)
	daysUntilFriday := (int(time.Friday) - int(localTime.Weekday()) + 7) % 7
	if daysUntilFriday == 0 {
		daysUntilFriday = 7
	}
	return mattermostScenarioRuntimeValues{
		NextFriday: localTime.AddDate(0, 0, daysUntilFriday).Format("2006-01-02"),
		Tomorrow:   localTime.AddDate(0, 0, 1).Format("2006-01-02"),
	}
}

func replaceMattermostScenarioRuntimeValues(value string, runtimeValues mattermostScenarioRuntimeValues) string {
	value = strings.ReplaceAll(value, "{{nextFriday}}", runtimeValues.NextFriday)
	return strings.ReplaceAll(value, "{{tomorrow}}", runtimeValues.Tomorrow)
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
	for toolName, count := range step.ExpectedExactToolCallCounts {
		if strings.TrimSpace(toolName) == "" || count < 0 {
			return fmt.Errorf("Mattermost scenario step %d has invalid exact tool call count", stepIndex)
		}
	}
	for eventIndex, eventCount := range step.ExpectedEventCounts {
		if (strings.TrimSpace(eventCount.Name) == "" && len(eventCount.AnyOfNames) == 0) || eventCount.Count < 0 {
			return fmt.Errorf("Mattermost scenario step %d event count %d is invalid", stepIndex, eventIndex)
		}
		for _, eventName := range eventCount.eventNames() {
			if eventCount.OutputFragment != "" && !isMattermostScenarioToolResultEvent(eventName) {
				return fmt.Errorf("Mattermost scenario step %d event count %d outputFragment requires a tool result event", stepIndex, eventIndex)
			}
		}
		if eventCount.BodyFragment != "" && eventCount.OutputFragment != "" {
			return fmt.Errorf("Mattermost scenario step %d event count %d cannot combine bodyFragment and outputFragment", stepIndex, eventIndex)
		}
	}
	return nil
}

func isMattermostScenarioToolResultEvent(name string) bool {
	_, isDirectToolEvent := mattermostScenarioDirectToolName(name, "result")
	return isDirectToolEvent
}

func mattermostScenarioDirectToolName(eventName string, phase string) (string, bool) {
	prefix := "tool."
	suffix := "." + phase
	if !strings.HasPrefix(eventName, prefix) || !strings.HasSuffix(eventName, suffix) {
		return "", false
	}
	toolName := strings.TrimSuffix(strings.TrimPrefix(eventName, prefix), suffix)
	if toolName == "" || toolName == "capability.invoke" || toolName == "task.history" {
		return "", false
	}
	return toolName, true
}

func validateMattermostScenarioResult(scenario mattermostScenario, result *mattermostScenarioResult) error {
	if result.ScenarioName != scenario.Name {
		return fmt.Errorf("Mattermost scenario result name %q does not match %q", result.ScenarioName, scenario.Name)
	}
	if len(result.Steps) != len(scenario.Steps) {
		return fmt.Errorf("Mattermost scenario returned %d steps, expected %d", len(result.Steps), len(scenario.Steps))
	}
	for stepIndex, expectedStep := range scenario.Steps {
		if scenario.RequiresLLMD {
			if errorValue := validateMattermostScenarioLLMD(stepIndex, result.Steps[stepIndex].TaskEvents, scenario); errorValue != nil {
				return errorValue
			}
		}
		if errorValue := validateMattermostScenarioStep(stepIndex, scenario, expectedStep, result.Steps[stepIndex], result); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func validateMattermostScenarioStep(stepIndex int, scenario mattermostScenario, expected mattermostScenarioStep, actual mattermostScenarioStepResult, result *mattermostScenarioResult) error {
	if actual.Prompt != expected.Prompt {
		return fmt.Errorf("Mattermost scenario step %d prompt does not match", stepIndex)
	}
	if expected.ExpectedTaskStatus != "" && actual.TaskStatus != expected.ExpectedTaskStatus {
		return fmt.Errorf("Mattermost scenario step %d status %q does not match %q", stepIndex, actual.TaskStatus, expected.ExpectedTaskStatus)
	}
	if scenarioHasExposureExpectations(scenario) {
		if errorValue := validateMattermostScenarioExposure(stepIndex, scenario, expected, actual.TaskEvents); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := validateMattermostScenarioReply(stepIndex, expected, actual.BotMessage); errorValue != nil {
		return errorValue
	}
	for _, attachmentName := range expected.ExpectedAttachments {
		if !hasMattermostScenarioAttachment(actual.Attachments, attachmentName) {
			return fmt.Errorf("Mattermost scenario step %d is missing attachment %q", stepIndex, attachmentName)
		}
	}
	if errorValue := validateMattermostScenarioAttachmentText(stepIndex, expected, actual.Attachments); errorValue != nil {
		return errorValue
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
		if strings.TrimSpace(actual.WorkspaceEvidenceError) == "" {
			return errorValue
		}
		recordMattermostScenarioAdvisoryFailure(result, stepIndex, "workspace_files", errorValue)
	}
	return validateMattermostScenarioEvents(stepIndex, expected, actual.TaskEvents, result)
}

func validateMattermostScenarioAttachmentText(stepIndex int, expected mattermostScenarioStep, files []downloadedMattermostFile) error {
	if len(expected.ExpectedAttachmentText) == 0 {
		return nil
	}
	for _, file := range files {
		if !hasMattermostScenarioAttachment([]downloadedMattermostFile{file}, strings.Join(expected.ExpectedAttachments, "")) {
			continue
		}
		content, errorValue := base64.StdEncoding.DecodeString(file.ContentBase64)
		if errorValue != nil {
			return fmt.Errorf("Mattermost scenario step %d decode attachment: %w", stepIndex, errorValue)
		}
		if strings.HasSuffix(strings.ToLower(file.Filename), ".json") && !json.Valid(content) {
			return fmt.Errorf("Mattermost scenario step %d attachment %q is not valid JSON", stepIndex, file.Filename)
		}
		for _, expectedText := range expected.ExpectedAttachmentText {
			if !bytes.Contains(content, []byte(expectedText)) {
				return fmt.Errorf("Mattermost scenario step %d attachment %q is missing %q", stepIndex, file.Filename, expectedText)
			}
		}
		return nil
	}
	return fmt.Errorf("Mattermost scenario step %d has no attachment to inspect", stepIndex)
}

func scenarioHasExposureExpectations(scenario mattermostScenario) bool {
	return len(scenario.AllowedTools) > 0 || len(scenario.InitialToolNames) > 0 || len(scenario.SkillDirectoryPaths) > 0
}

func validateMattermostScenarioExposure(stepIndex int, scenario mattermostScenario, expected mattermostScenarioStep, events []mattermostScenarioTaskEvent) error {
	var instruction mattermostScenarioInstructionEvidence
	var workingSet mattermostScenarioWorkingSetEvidence
	hasInstruction := false
	hasWorkingSet := false
	for _, event := range events {
		switch event.Name {
		case "agent.instructions_loaded":
			if errorValue := json.Unmarshal([]byte(event.Body), &instruction); errorValue != nil {
				return fmt.Errorf("Mattermost scenario step %d has invalid instruction exposure evidence: %w", stepIndex, errorValue)
			}
			hasInstruction = true
		case "agent.step_working_set":
			if errorValue := json.Unmarshal([]byte(event.Body), &workingSet); errorValue != nil {
				return fmt.Errorf("Mattermost scenario step %d has invalid working set exposure evidence: %w", stepIndex, errorValue)
			}
			hasWorkingSet = true
		}
	}
	if !hasInstruction || !hasWorkingSet {
		return fmt.Errorf("Mattermost scenario step %d is missing direct-tool exposure evidence", stepIndex)
	}
	if errorValue := validateMattermostScenarioForbiddenTools(stepIndex, instruction.ExposedToolNames, workingSet.Exposure.ExposedToolIDs); errorValue != nil {
		return errorValue
	}
	if workingSet.Exposure.UsedFallbackGroups || workingSet.Exposure.SelectionSource == "deterministic_palette" {
		return fmt.Errorf("Mattermost scenario step %d used fallback or deterministic tool exposure", stepIndex)
	}
	if stepIndex == 0 {
		if errorValue := validateMattermostScenarioInitialTools(stepIndex, scenario, workingSet.Exposure); errorValue != nil {
			return errorValue
		}
		if errorValue := validateMattermostScenarioSelectedSkills(stepIndex, scenario, instruction); errorValue != nil {
			return errorValue
		}
	}
	for _, toolName := range append(append([]string{}, expected.ExpectedToolCalls...), expected.ExpectedAnyToolCalls...) {
		if len(scenario.AllowedTools) > 0 && !containsMattermostScenarioString(scenario.AllowedTools, toolName) {
			return fmt.Errorf("Mattermost scenario step %d expected tool %q is not in allowedTools", stepIndex, toolName)
		}
		if !containsMattermostScenarioString(workingSet.Exposure.ExposedToolIDs, toolName) {
			return fmt.Errorf("Mattermost scenario step %d expected tool %q was not exposed", stepIndex, toolName)
		}
	}
	return nil
}

func validateMattermostScenarioForbiddenTools(stepIndex int, instructionTools []string, workingSetTools []string) error {
	for _, toolName := range append(append([]string{}, instructionTools...), workingSetTools...) {
		if toolName == "capability.invoke" || toolName == "task.history" {
			return fmt.Errorf("Mattermost scenario step %d exposed internal tool %q", stepIndex, toolName)
		}
	}
	return nil
}

func validateMattermostScenarioInitialTools(stepIndex int, scenario mattermostScenario, exposure mattermostScenarioExposureEvidence) error {
	if len(scenario.InitialToolNames) == 0 {
		return nil
	}
	hasExposedInitialTool := false
	for _, toolName := range scenario.InitialToolNames {
		if len(scenario.AllowedTools) > 0 && !containsMattermostScenarioString(scenario.AllowedTools, toolName) {
			return fmt.Errorf("Mattermost scenario step %d initial tool %q is not in allowedTools", stepIndex, toolName)
		}
		if containsMattermostScenarioString(exposure.ExposedToolIDs, toolName) {
			hasExposedInitialTool = true
		}
	}
	if !hasExposedInitialTool {
		return fmt.Errorf("Mattermost scenario step %d did not expose any initial tool of %q; the contract working set selected the wrong namespace", stepIndex, strings.Join(scenario.InitialToolNames, ", "))
	}
	return nil
}

func validateMattermostScenarioSelectedSkills(stepIndex int, scenario mattermostScenario, instruction mattermostScenarioInstructionEvidence) error {
	for _, skillPath := range scenario.SkillDirectoryPaths {
		skillName := path.Base(path.Clean(skillPath))
		if skillName == "." || skillName == "/" || skillName == "" {
			continue
		}
		if _, isSelected := instruction.SelectedSkillToolReferences[skillName]; !isSelected {
			return fmt.Errorf("Mattermost scenario step %d did not select skill %q", stepIndex, skillName)
		}
	}
	return nil
}

func containsMattermostScenarioString(values []string, expected string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(expected) {
			return true
		}
	}
	return false
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
		if errorValue := recordMattermostScenarioCount(result, stepIndex, "tool", toolName, expectedCount, observedCount, false); errorValue != nil {
			return errorValue
		}
	}
	for toolName, expectedCount := range expected.ExpectedExactToolCallCounts {
		observedCount := countMattermostScenarioToolEvents(events, toolName)
		if errorValue := recordMattermostScenarioCount(result, stepIndex, "tool", toolName, expectedCount, observedCount, true); errorValue != nil {
			return errorValue
		}
	}
	for _, eventCount := range expected.ExpectedEventCounts {
		observedCount := countMattermostScenarioExpectedEvents(events, eventCount)
		if eventCount.Count > 0 && observedCount == 0 {
			missingEventError := missingMattermostScenarioEventError(stepIndex, eventCount)
			if eventCount.Advisory {
				recordMattermostScenarioAdvisoryFailure(result, stepIndex, eventCount.Name, missingEventError)
				continue
			}
			return missingEventError
		}
		if errorValue := recordMattermostScenarioCount(result, stepIndex, "event", eventCount.Name, eventCount.Count, observedCount, eventCount.Exact); errorValue != nil {
			if eventCount.Advisory {
				recordMattermostScenarioAdvisoryFailure(result, stepIndex, eventCount.Name, errorValue)
				continue
			}
			return errorValue
		}
	}
	if errorValue := validateMattermostScenarioCalendarMutationIDs(stepIndex, expected, events); errorValue != nil {
		return errorValue
	}
	return validateMattermostScenarioMessageMutationIDs(stepIndex, expected, events, result)
}

func validateMattermostScenarioCalendarMutationIDs(stepIndex int, expected mattermostScenarioStep, events []mattermostScenarioTaskEvent) error {
	if !containsMattermostScenarioString(expected.ExpectedToolCalls, "calendar.list") {
		return nil
	}
	for eventIndex, event := range events {
		toolName, isMutation := mattermostScenarioCalendarMutationName(event.Name)
		if !isMutation || !containsMattermostScenarioString(expected.ExpectedToolCalls, toolName) {
			continue
		}
		eventHint := mattermostScenarioCalendarMutationEventHint(event.Body)
		if eventHint == "" {
			return fmt.Errorf("Mattermost scenario step %d %s request has no eventHint", stepIndex, toolName)
		}
		if !listedMattermostScenarioCalendarEventHints(events[:eventIndex])[eventHint] {
			return fmt.Errorf("Mattermost scenario step %d %s used eventHint %q before calendar.list returned it", stepIndex, toolName, eventHint)
		}
	}
	return nil
}

func mattermostScenarioCalendarMutationName(eventName string) (string, bool) {
	for _, toolName := range []string{"calendar.update", "calendar.delete"} {
		if eventName == "tool."+toolName+".requested" {
			return toolName, true
		}
	}
	return "", false
}

func mattermostScenarioCalendarMutationEventHint(body string) string {
	var request struct {
		Input struct {
			EventHint string `json:"eventHint"`
		} `json:"input"`
	}
	if json.Unmarshal([]byte(body), &request) != nil {
		return ""
	}
	return strings.TrimSpace(request.Input.EventHint)
}

func listedMattermostScenarioCalendarEventHints(events []mattermostScenarioTaskEvent) map[string]bool {
	eventHints := map[string]bool{}
	for _, event := range events {
		for _, eventHint := range mattermostScenarioCalendarListEventHints(event) {
			eventHints[eventHint] = true
		}
	}
	return eventHints
}

func mattermostScenarioCalendarListEventHints(event mattermostScenarioTaskEvent) []string {
	if event.Name != "tool.calendar.list.result" {
		return nil
	}
	var result struct {
		Events []struct {
			EventID string `json:"eventID"`
			Title   string `json:"title"`
		} `json:"events"`
	}
	if json.Unmarshal(mattermostScenarioToolResultData(event), &result) != nil {
		return nil
	}
	eventHints := make([]string, 0, len(result.Events)*2)
	for _, eventValue := range result.Events {
		if eventID := strings.TrimSpace(eventValue.EventID); eventID != "" {
			eventHints = append(eventHints, eventID)
		}
		if title := strings.TrimSpace(eventValue.Title); title != "" {
			eventHints = append(eventHints, title)
		}
	}
	return eventHints
}

func validateMattermostScenarioMessageMutationIDs(stepIndex int, expected mattermostScenarioStep, events []mattermostScenarioTaskEvent, result *mattermostScenarioResult) error {
	lineageIDs := priorMattermostScenarioMessageIDs(stepIndex, result)
	searchIDs := map[string]bool{}
	for eventIndex, event := range events {
		if event.Name == "tool.message.search.result" {
			addMattermostScenarioMessageIDs(searchIDs, mattermostScenarioMessageResultIDs(event))
			continue
		}
		toolName, isMutation := mattermostScenarioMessageMutationName(event.Name)
		if !isMutation || !containsMattermostScenarioString(expected.ExpectedToolCalls, toolName) {
			continue
		}
		messageIDs := mattermostScenarioMessageMutationRequestIDs(event)
		if len(messageIDs) == 0 {
			return fmt.Errorf("Mattermost scenario step %d %s request has no message ID", stepIndex, toolName)
		}
		for _, messageID := range messageIDs {
			if containsMattermostScenarioString(expected.ExpectedToolCalls, "message.search") && !searchIDs[messageID] {
				return fmt.Errorf("Mattermost scenario step %d %s used message ID %q before message.search returned it at event %d", stepIndex, toolName, messageID, eventIndex)
			}
			if len(lineageIDs) > 0 && !lineageIDs[messageID] {
				return fmt.Errorf("Mattermost scenario step %d %s used message ID %q outside the scenario mutation lineage", stepIndex, toolName, messageID)
			}
		}
	}
	return nil
}

func mattermostScenarioMessageMutationName(eventName string) (string, bool) {
	for _, toolName := range []string{"message.update", "message.delete"} {
		if eventName == "tool."+toolName+".requested" {
			return toolName, true
		}
	}
	return "", false
}

func mattermostScenarioMessageMutationRequestIDs(event mattermostScenarioTaskEvent) []string {
	var request struct {
		Input struct {
			MessageID  string   `json:"messageID"`
			MessageIDs []string `json:"messageIDs"`
		} `json:"input"`
	}
	if json.Unmarshal([]byte(event.Body), &request) != nil {
		return nil
	}
	messageIDs := append([]string{}, request.Input.MessageIDs...)
	if messageID := strings.TrimSpace(request.Input.MessageID); messageID != "" {
		messageIDs = append(messageIDs, messageID)
	}
	return trimmedNonEmptyValues(messageIDs)
}

func priorMattermostScenarioMessageIDs(stepIndex int, result *mattermostScenarioResult) map[string]bool {
	messageIDs := map[string]bool{}
	if result == nil || stepIndex <= 0 {
		return messageIDs
	}
	for _, step := range result.Steps[:min(stepIndex, len(result.Steps))] {
		for _, event := range step.TaskEvents {
			if event.Name != "tool.message.send.result" && event.Name != "tool.message.update.result" {
				continue
			}
			addMattermostScenarioMessageIDs(messageIDs, mattermostScenarioMessageResultIDs(event))
		}
	}
	return messageIDs
}

func addMattermostScenarioMessageIDs(destination map[string]bool, messageIDs []string) {
	for _, messageID := range messageIDs {
		if normalizedMessageID := strings.TrimSpace(messageID); normalizedMessageID != "" {
			destination[normalizedMessageID] = true
		}
	}
}

func mattermostScenarioMessageResultIDs(event mattermostScenarioTaskEvent) []string {
	var result struct {
		MessageID  string   `json:"messageID"`
		MessageIDs []string `json:"messageIDs"`
	}
	if json.Unmarshal(mattermostScenarioToolResultData(event), &result) != nil {
		return nil
	}
	messageIDs := append([]string{}, result.MessageIDs...)
	if messageID := strings.TrimSpace(result.MessageID); messageID != "" {
		messageIDs = append(messageIDs, messageID)
	}
	return trimmedNonEmptyValues(messageIDs)
}

func mattermostScenarioToolResultData(event mattermostScenarioTaskEvent) json.RawMessage {
	var observation struct {
		Output struct {
			Content string          `json:"content"`
			Data    json.RawMessage `json:"data"`
		} `json:"output"`
	}
	if json.Unmarshal([]byte(event.Body), &observation) != nil {
		return nil
	}
	if len(bytes.TrimSpace(observation.Output.Data)) > 0 {
		return observation.Output.Data
	}
	return json.RawMessage(strings.TrimSpace(observation.Output.Content))
}

func missingMattermostScenarioEventError(stepIndex int, expectation mattermostScenarioEventCount) error {
	if expectation.OutputFragment != "" {
		return fmt.Errorf("Mattermost scenario step %d is missing event %q with output containing %q", stepIndex, expectation.eventNamesLabel(), expectation.OutputFragment)
	}
	if expectation.BodyFragment != "" {
		return fmt.Errorf("Mattermost scenario step %d is missing event %q with body containing %q", stepIndex, expectation.eventNamesLabel(), expectation.BodyFragment)
	}
	return fmt.Errorf("Mattermost scenario step %d is missing event %q", stepIndex, expectation.eventNamesLabel())
}

func validateMattermostScenarioLLMD(stepIndex int, events []mattermostScenarioTaskEvent, scenario mattermostScenario) error {
	requiredSchemaNames := []string{"blueclaw_turn_router", "blueclaw_agent_turn_action"}
	authoritativeSchemaNameSet := testStringSet([]string{
		"blueclaw_agent_turn_action",
		"blueclaw_agent_turn_finalizer",
		"blueclaw_turn_router",
		"blueclaw_recovery_decision",
		"blueclaw_operation_contract",
	})
	successfulSchemaNames := map[string]bool{}
	for _, event := range events {
		if event.Name != "llm.call" {
			continue
		}
		var call struct {
			SchemaName      string `json:"schemaName"`
			Transport       string `json:"transport"`
			Provider        string `json:"provider"`
			Model           string `json:"model"`
			ModelTier       string `json:"modelTier"`
			SelectedBackend string `json:"selectedBackend"`
			UsedFallback    bool   `json:"usedFallback"`
			IsError         bool   `json:"isError"`
		}
		if json.Unmarshal([]byte(event.Body), &call) != nil {
			continue
		}
		if !authoritativeSchemaNameSet[call.SchemaName] {
			continue
		}
		if call.Transport != "llmd" {
			return fmt.Errorf("Mattermost scenario step %d used %s transport for authoritative AI SDK call %s", stepIndex, call.Transport, call.SchemaName)
		}
		if call.IsError {
			continue
		}
		if strings.TrimSpace(call.Provider) == "" || strings.TrimSpace(call.Model) == "" || strings.TrimSpace(call.SelectedBackend) == "" {
			return fmt.Errorf("Mattermost scenario step %d has incomplete model provenance for authoritative AI SDK call %s", stepIndex, call.SchemaName)
		}
		if !isMattermostScenarioModelTierAtOrBelow(call.ModelTier, scenario.MaximumModelTier) {
			return fmt.Errorf("Mattermost scenario step %d authoritative AI SDK call %s reported model tier %q above maximum %q", stepIndex, call.SchemaName, call.ModelTier, scenario.MaximumModelTier)
		}
		if scenario.ExpectedLLMProvider != "" && call.Provider != scenario.ExpectedLLMProvider {
			return fmt.Errorf("Mattermost scenario step %d authoritative AI SDK call %s selected provider %q, expected %q", stepIndex, call.SchemaName, call.Provider, scenario.ExpectedLLMProvider)
		}
		successfulSchemaNames[call.SchemaName] = true
	}
	for _, schemaName := range requiredSchemaNames {
		if !successfulSchemaNames[schemaName] {
			return fmt.Errorf("Mattermost scenario step %d has no successful authoritative AI SDK call for %s", stepIndex, schemaName)
		}
	}
	return nil
}

func isMattermostScenarioModelTierAtOrBelow(modelTier string, maximumModelTier string) bool {
	maximumRank, hasMaximum := mattermostScenarioModelTierRank(maximumModelTier)
	if !hasMaximum {
		return strings.TrimSpace(maximumModelTier) == ""
	}
	modelRank, hasModel := mattermostScenarioModelTierRank(modelTier)
	return hasModel && modelRank <= maximumRank
}

func mattermostScenarioModelTierRank(modelTier string) (int, bool) {
	for rank, name := range []string{"xlow", "low", "medium", "high", "xhigh", "max"} {
		if strings.EqualFold(strings.TrimSpace(modelTier), name) {
			return rank, true
		}
	}
	return 0, false
}

func hasAnyMattermostScenarioToolEvent(events []mattermostScenarioTaskEvent, toolNames []string) bool {
	for _, toolName := range toolNames {
		if countMattermostScenarioToolEvents(events, toolName) > 0 {
			return true
		}
	}
	return false
}

func recordMattermostScenarioCount(result *mattermostScenarioResult, stepIndex int, kind string, name string, expected int, observed int, isExact bool) error {
	if expected == 0 && observed != 0 {
		if isExact {
			return fmt.Errorf("Mattermost scenario step %d has forbidden %s %q count %d", stepIndex, kind, name, observed)
		}
		recordMattermostScenarioEfficiencyObservation(result, mattermostScenarioEfficiencyObservation{
			StepIndex: stepIndex,
			Kind:      kind,
			Name:      name,
			Expected:  expected,
			Observed:  observed,
		})
		return nil
	}
	if expected > 0 && observed == 0 {
		return fmt.Errorf("Mattermost scenario step %d is missing %s %q", stepIndex, kind, name)
	}
	if expected > 0 && observed != expected {
		if isExact {
			return fmt.Errorf("Mattermost scenario step %d has exact %s %q count %d, expected %d", stepIndex, kind, name, observed, expected)
		}
		recordMattermostScenarioEfficiencyObservation(result, mattermostScenarioEfficiencyObservation{
			StepIndex: stepIndex,
			Kind:      kind,
			Name:      name,
			Expected:  expected,
			Observed:  observed,
		})
	}
	return nil
}

func recordMattermostScenarioAdvisoryFailure(result *mattermostScenarioResult, stepIndex int, name string, errorValue error) {
	if result == nil || errorValue == nil {
		return
	}
	result.AdvisoryFailures = append(result.AdvisoryFailures, mattermostScenarioAdvisoryFailure{
		StepIndex: stepIndex,
		Name:      name,
		Reason:    errorValue.Error(),
	})
}

func printMattermostScenarioAdvisoryWarnings(result mattermostScenarioResult) {
	for _, failure := range result.AdvisoryFailures {
		fmt.Printf("warning: advisory expectation %q at step %d did not hold: %s\n", failure.Name, failure.StepIndex, failure.Reason)
	}
}

func recordMattermostScenarioEfficiencyObservation(result *mattermostScenarioResult, observation mattermostScenarioEfficiencyObservation) {
	for observationIndex := range result.EfficiencyObservations {
		existing := result.EfficiencyObservations[observationIndex]
		if existing.StepIndex == observation.StepIndex && existing.Kind == observation.Kind && existing.Name == observation.Name {
			result.EfficiencyObservations[observationIndex] = observation
			return
		}
	}
	result.EfficiencyObservations = append(result.EfficiencyObservations, observation)
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
	count := 0
	for _, event := range events {
		if !expected.matchesEventName(event.Name) {
			continue
		}
		if expected.OutputFragment != "" {
			if toolResultOutputContains(event.Body, expected.OutputFragment) {
				count++
			}
			continue
		}
		if strings.Contains(event.Body, expected.BodyFragment) {
			count++
		}
	}
	return count
}

func toolResultOutputContains(body string, fragment string) bool {
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
		if event.Name == "tool."+toolName+".requested" {
			count++
		}
	}
	return count
}
