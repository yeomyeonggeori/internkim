package capabilityprotocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	bluecollarTaskEventNamePath = "../../.dependency/blueclaw/.dependency/blueprotocol/agentcontract/task_event_name.go"
	bluecollarKernelToolsPath   = "../../.dependency/blueclaw/.dependency/blueprotocol/toolcontract/kernel_tools.go"
	expensiveScenarioDirectory  = "../../tests/expensive"
	scenarioEventCountField     = "expectedEventCounts"
	scenarioEventListField      = "expectedEvents"
)

var blueclawHostTaskEventNamePaths = []string{
	"../../.dependency/blueclaw/internal/task/host_task_event_names.go",
	"../../.dependency/blueclaw/internal/approvalgate/permission_asker.go",
	"../../.dependency/blueclaw/internal/approvalgate/wording.go",
}

var taskEventNameConsumerPaths = []string{
	"../../web/src/routes/runs/runs-api.ts",
}

var (
	goStringConstant     = regexp.MustCompile(`(?m)^\s*(?:const\s+)?([A-Za-z][A-Za-z0-9]*)\s+=\s+"([^"]*)"`)
	quotedEventCandidate = regexp.MustCompile(`['"]([a-z.][a-zA-Z0-9_.]*)['"]`)
)

func TestDeclaredTaskEventNamesMatchBluecollar(t *testing.T) {
	canonicalNames := []string{}
	for _, sourcePath := range append([]string{bluecollarTaskEventNamePath}, blueclawHostTaskEventNamePaths...) {
		for identifier, value := range bluecollarStringConstants(t, sourcePath) {
			if strings.HasPrefix(identifier, "TaskEvent") {
				canonicalNames = append(canonicalNames, value)
			}
		}
	}
	sort.Strings(canonicalNames)
	if len(canonicalNames) == 0 {
		t.Fatalf("no task event names found in %s", bluecollarTaskEventNamePath)
	}

	if !reflect.DeepEqual(canonicalNames, DeclaredTaskEventNames()) {
		t.Fatalf("the generated task-event-name schema drifted from bluecollar: canonical=%d generated=%d", len(canonicalNames), len(DeclaredTaskEventNames()))
	}
}

func TestDeclaredToolTaskEventSuffixesMatchBluecollar(t *testing.T) {
	constants := bluecollarStringConstants(t, bluecollarTaskEventNamePath)
	canonicalSuffixes := []string{
		constants["ToolTaskEventRequestedSuffix"],
		constants["ToolTaskEventResultSuffix"],
		constants["ToolTaskEventCancelledSuffix"],
	}
	for _, suffix := range canonicalSuffixes {
		if suffix == "" {
			t.Fatalf("a tool task event suffix is missing from %s", bluecollarTaskEventNamePath)
		}
	}

	sort.Strings(canonicalSuffixes)
	generatedSuffixes := DeclaredToolTaskEventSuffixes()
	sort.Strings(generatedSuffixes)
	if !reflect.DeepEqual(canonicalSuffixes, generatedSuffixes) {
		t.Fatalf("the generated tool-task-event-suffix schema drifted from bluecollar: canonical=%v generated=%v", canonicalSuffixes, generatedSuffixes)
	}
}

func TestTheMirroredTaskEventNameIsDeclared(t *testing.T) {
	if !IsDeclaredTaskEventName(TaskEventConfirmationRequested) {
		t.Fatalf("%q is mirrored here but no producer declares it", TaskEventConfirmationRequested)
	}
}

func TestEveryExpensiveScenarioEventNamesAnEmittedEvent(t *testing.T) {
	knownToolNames := knownToolNames(t)
	scenarioPaths, errorValue := filepath.Glob(filepath.Join(expensiveScenarioDirectory, "*.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(scenarioPaths) == 0 {
		t.Fatalf("no expensive scenarios found in %s", expensiveScenarioDirectory)
	}

	for _, scenarioPath := range scenarioPaths {
		for _, eventName := range scenarioEventNames(t, scenarioPath) {
			if !IsDeclaredTaskEventName(eventName) {
				t.Errorf("%s expects %q, which no producer emits", filepath.Base(scenarioPath), eventName)
				continue
			}
			toolName, isToolEvent := ToolTaskEventToolName(eventName)
			if isToolEvent && !knownToolNames[toolName] {
				t.Errorf("%s expects %q, and no tool is named %q", filepath.Base(scenarioPath), eventName, toolName)
			}
		}
	}
}

func TestEveryConsumerNamesTheDeclaredVocabulary(t *testing.T) {
	declaredNames := DeclaredTaskEventNames()
	for _, consumerPath := range taskEventNameConsumerPaths {
		source, errorValue := os.ReadFile(filepath.FromSlash(consumerPath))
		if errorValue != nil {
			t.Fatalf("reading %s: %v", consumerPath, errorValue)
		}
		for _, candidate := range eventCandidates(string(source)) {
			if namesTheDeclaredVocabulary(candidate, declaredNames) {
				continue
			}
			t.Errorf("%s names %q, which is neither a declared event name nor part of the tool event grammar", filepath.Base(consumerPath), candidate)
		}
	}
}

func eventCandidates(source string) []string {
	candidates := []string{}
	seen := map[string]bool{}
	firstSegments := declaredTaskEventFirstSegments()
	for _, match := range quotedEventCandidate.FindAllStringSubmatch(source, -1) {
		candidate := match[1]
		if !strings.Contains(candidate, ".") || seen[candidate] {
			continue
		}
		if !firstSegments[strings.Split(candidate, ".")[0]] {
			continue
		}
		seen[candidate] = true
		candidates = append(candidates, candidate)
	}
	sort.Strings(candidates)
	return candidates
}

func namesTheDeclaredVocabulary(candidate string, declaredNames []string) bool {
	if IsDeclaredTaskEventName(candidate) {
		return true
	}
	for _, declaredName := range declaredNames {
		if strings.HasPrefix(declaredName, candidate) {
			return true
		}
	}
	return strings.Contains(toolTaskEventNameMatch.String(), regexp.QuoteMeta(candidate))
}

func declaredTaskEventFirstSegments() map[string]bool {
	segments := map[string]bool{}
	for _, declaredName := range DeclaredTaskEventNames() {
		segments[strings.Split(declaredName, ".")[0]] = true
	}
	return segments
}

func scenarioEventNames(t *testing.T, scenarioPath string) []string {
	t.Helper()
	source, errorValue := os.ReadFile(scenarioPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	document := any(nil)
	if errorValue := json.Unmarshal(source, &document); errorValue != nil {
		t.Fatalf("reading %s: %v", scenarioPath, errorValue)
	}
	names := []string{}
	collectScenarioEventNames(document, "", &names)
	sort.Strings(names)
	return names
}

func collectScenarioEventNames(node any, containerFieldName string, names *[]string) {
	switch value := node.(type) {
	case map[string]any:
		if containerFieldName == scenarioEventCountField {
			if name, isString := value["name"].(string); isString {
				*names = append(*names, name)
			}
		}
		for key, child := range value {
			collectScenarioEventNames(child, key, names)
		}
	case []any:
		for _, child := range value {
			collectScenarioEventNames(child, containerFieldName, names)
		}
	case string:
		if containerFieldName == scenarioEventListField {
			*names = append(*names, value)
		}
	}
}

func knownToolNames(t *testing.T) map[string]bool {
	t.Helper()
	names := map[string]bool{}
	for _, descriptor := range GeneratedToolDescriptorSet() {
		names[descriptor.Name] = true
	}
	for identifier, value := range bluecollarStringConstants(t, bluecollarKernelToolsPath) {
		if strings.HasSuffix(identifier, "ToolName") {
			names[value] = true
		}
	}
	if len(names) == 0 {
		t.Fatal("no tool names found, so a green scenario check would mean nothing")
	}
	return names
}

func bluecollarStringConstants(t *testing.T, sourcePath string) map[string]string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.FromSlash(sourcePath))
	if errorValue != nil {
		t.Fatalf("blueprotocol source unavailable: %v", errorValue)
	}
	constants := map[string]string{}
	for _, match := range goStringConstant.FindAllStringSubmatch(string(source), -1) {
		constants[match[1]] = match[2]
	}
	return constants
}
