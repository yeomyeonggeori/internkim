package capabilities

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

// Captured from this package at internkim@a22c8b61a, the commit before the
// catalog moved.
const descriptorSetSnapshotPath = "testdata/descriptor-sets-before-the-catalog-move.json"

type descriptorSetSnapshot struct {
	Membership map[string][]string   `json:"membership"`
	Shared     map[string]Descriptor `json:"shared"`
	PerSet     map[string]Descriptor `json:"perSet"`
}

func (snapshot descriptorSetSnapshot) descriptorFor(setName string, toolName string) (Descriptor, bool) {
	if descriptor, isFound := snapshot.PerSet[setName+"/"+toolName]; isFound {
		return descriptor, true
	}
	descriptor, isFound := snapshot.Shared[toolName]
	return descriptor, isFound
}

func readDescriptorSetSnapshot(t *testing.T) descriptorSetSnapshot {
	t.Helper()
	document, errorValue := os.ReadFile(descriptorSetSnapshotPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var snapshot descriptorSetSnapshot
	if errorValue := json.Unmarshal(document, &snapshot); errorValue != nil {
		t.Fatal(errorValue)
	}
	return snapshot
}

func TestEverySetPublishesWhatItPublishedBeforeTheCatalogMove(t *testing.T) {
	snapshot := readDescriptorSetSnapshot(t)
	sets := map[string][]Descriptor{
		"CompanionLLMDescriptors":    CompanionLLMDescriptors(),
		"CompanionToolDescriptors":   CompanionToolDescriptors(),
		"DefaultToolDescriptors":     DefaultToolDescriptors(),
		"DeviceBrowserDescriptors":   DeviceBrowserDescriptors(),
		"DeviceDescriptors":          DeviceDescriptors(),
		"GoogleWorkspaceDescriptors": GoogleWorkspaceDescriptors(),
		"RegisteredToolDescriptors":  RegisteredToolDescriptors(),
	}

	if len(sets) != len(snapshot.Membership) {
		t.Fatalf("the snapshot holds %d sets and this test builds %d", len(snapshot.Membership), len(sets))
	}

	for setName, descriptors := range sets {
		t.Run(setName, func(t *testing.T) {
			actualNames := make([]string, 0, len(descriptors))
			for _, descriptor := range descriptors {
				actualNames = append(actualNames, descriptor.Name)
			}
			expectedNames := snapshot.Membership[setName]
			if !reflect.DeepEqual(sortedCopy(expectedNames), sortedCopy(actualNames)) {
				t.Fatalf("the set held %v, it now holds %v", expectedNames, actualNames)
			}
			for _, descriptor := range descriptors {
				expected, isFound := snapshot.descriptorFor(setName, descriptor.Name)
				if !isFound {
					t.Errorf("the snapshot holds no %s for %s", descriptor.Name, setName)
					continue
				}
				t.Run(descriptor.Name, func(t *testing.T) {
					expectSameDescriptor(t, expected, descriptor)
				})
			}
		})
	}
}

func sortedCopy(names []string) []string {
	sorted := append([]string{}, names...)
	sort.Strings(sorted)
	return sorted
}

func expectSameDescriptor(t *testing.T, expected Descriptor, actual Descriptor) {
	t.Helper()
	expectSameField(t, "canonicalName", expected.CanonicalName, actual.CanonicalName)
	expectSameField(t, "namespace", expected.Namespace, actual.Namespace)
	expectSameField(t, "answeredBy", expected.AnsweredBy, actual.AnsweredBy)
	expectSameField(t, "modelName", expected.ModelName, actual.ModelName)
	expectSameField(t, "modelVisibility", expected.ModelVisibility, actual.ModelVisibility)
	expectSameField(t, "modelVisible", expected.ModelVisible, actual.ModelVisible)
	expectSameField(t, "description", expected.Description, actual.Description)
	expectSameField(t, "version", expected.Version, actual.Version)
	expectSameField(t, "privacyClass", expected.PrivacyClass, actual.PrivacyClass)
	expectSameField(t, "estimatedLatency", expected.EstimatedLatency, actual.EstimatedLatency)
	expectSameField(t, "policyResource", expected.PolicyResource, actual.PolicyResource)
	expectSameField(t, "sideEffect", expected.SideEffect, actual.SideEffect)
	expectSameField(t, "sideEffectClass", expected.SideEffectClass, actual.SideEffectClass)
	expectSameField(t, "approvalScope", expected.ApprovalScope, actual.ApprovalScope)
	expectSameField(t, "requiresUserPresence", expected.RequiresUserPresence, actual.RequiresUserPresence)
	expectSameField(t, "requiresRequesterDevice", expected.RequiresRequesterDevice, actual.RequiresRequesterDevice)
	expectSameField(t, "requiresCompanionBrowser", expected.RequiresCompanionBrowser, actual.RequiresCompanionBrowser)
	expectSameField(t, "requiresApproval", expected.RequiresApproval, actual.RequiresApproval)
	expectSameField(t, "worksOffline", expected.WorksOffline, actual.WorksOffline)
	expectSameField(t, "idempotency", expected.Idempotency, actual.Idempotency)
	expectSameField(t, "completionEvidence", expected.CompletionEvidence, actual.CompletionEvidence)
	expectSameSchema(t, "inputSchema", expected.InputSchema, actual.InputSchema)
	expectSameSchema(t, "inputIntentSchema", expected.InputIntentSchema, actual.InputIntentSchema)
	expectSameSchema(t, "outputSchema", expected.OutputSchema, actual.OutputSchema)
	expectSameResultContract(t, expected.ResultContract, actual.ResultContract)
}

func expectSameResultContract(t *testing.T, expected *ToolResultContract, actual *ToolResultContract) {
	t.Helper()
	if expected == nil || actual == nil {
		if (expected == nil) != (actual == nil) {
			t.Errorf("resultContract: one side has a contract and the other does not")
		}
		return
	}
	expectSameSchema(t, "resultContract.schema", expected.Schema, actual.Schema)
	if len(expected.Effects) > 0 || len(actual.Effects) > 0 {
		expectSameField(t, "resultContract.effects", expected.Effects, actual.Effects)
	}
	expectSameField(t, "resultContract.evidenceCondition", expected.EvidenceCondition, actual.EvidenceCondition)
}

func expectSameField(t *testing.T, fieldName string, expected any, actual any) {
	t.Helper()
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("%s: it published %#v, it now publishes %#v", fieldName, expected, actual)
	}
}

func expectSameSchema(t *testing.T, fieldName string, expected json.RawMessage, actual json.RawMessage) {
	t.Helper()
	expectedValue, actualValue := decodeSnapshotSchema(t, expected), decodeSnapshotSchema(t, actual)
	if reflect.DeepEqual(expectedValue, actualValue) {
		return
	}
	for _, difference := range schemaDifferences(fieldName, expectedValue, actualValue) {
		t.Error(difference)
	}
}

func decodeSnapshotSchema(t *testing.T, document json.RawMessage) any {
	t.Helper()
	if len(document) == 0 {
		return nil
	}
	var value any
	if errorValue := json.Unmarshal(document, &value); errorValue != nil {
		t.Fatal(errorValue)
	}
	return value
}

const safeIntegerBound = float64(1<<53 - 1)

func isAuthoringKeyword(key string, expectedParent map[string]any, actualValue any) bool {
	if key == "$schema" {
		return true
	}
	if key != "minimum" && key != "maximum" {
		return false
	}
	if expectedParent["type"] != "integer" {
		return false
	}
	bound, isNumber := actualValue.(float64)
	return isNumber && (bound == safeIntegerBound || bound == -safeIntegerBound)
}

func schemaDifferences(path string, expected any, actual any) []string {
	expectedObject, expectedIsObject := expected.(map[string]any)
	actualObject, actualIsObject := actual.(map[string]any)
	if !expectedIsObject || !actualIsObject {
		if reflect.DeepEqual(expected, actual) {
			return nil
		}
		return []string{describeDifference(path, expected, actual)}
	}

	differences := []string{}
	for _, key := range sortedKeys(expectedObject, actualObject) {
		expectedChild, hasExpected := expectedObject[key]
		actualChild, hasActual := actualObject[key]
		switch {
		case !hasActual:
			differences = append(differences, path+"."+key+": it published this, it no longer does")
		case !hasExpected:
			if isAuthoringKeyword(key, expectedObject, actualChild) {
				continue
			}
			differences = append(differences, path+"."+key+": it publishes this now, it did not before")
		case key == "required":
			differences = append(differences, requiredDifference(path+"."+key, expectedChild, actualChild)...)
		default:
			differences = append(differences, schemaDifferences(path+"."+key, expectedChild, actualChild)...)
		}
	}
	return differences
}

func requiredDifference(path string, expected any, actual any) []string {
	expectedNames, actualNames := sortedStrings(expected), sortedStrings(actual)
	if reflect.DeepEqual(expectedNames, actualNames) {
		return nil
	}
	return []string{describeDifference(path, expectedNames, actualNames)}
}

func sortedStrings(value any) []string {
	values, isArray := value.([]any)
	if !isArray {
		return nil
	}
	names := make([]string, 0, len(values))
	for _, item := range values {
		name, isString := item.(string)
		if !isString {
			return nil
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func describeDifference(path string, expected any, actual any) string {
	expectedDocument, _ := json.Marshal(expected)
	actualDocument, _ := json.Marshal(actual)
	return path + ": it published " + string(expectedDocument) + ", it now publishes " + string(actualDocument)
}

func sortedKeys(left map[string]any, right map[string]any) []string {
	seen := map[string]bool{}
	keys := []string{}
	for key := range left {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range right {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}
