package jsonschema

import (
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The validator decides whether a document matches, and says so in the schema's
// own vocabulary: `validating root: validating /properties/tasks: type: null
// has type "null", want "array"`. That names a place in the schema, not a field
// in the document, and it never says what to send instead. The caller here is a
// model with one call to spend on recovery, and the run this was written for
// spent all of it rewriting an input that was already right.
//
// This walks the same schema over the same document and names what is wrong
// where the caller wrote it. It is a diagnosis, never a decision: the validator
// says whether, this says what, and when it finds nothing the validator's own
// sentence is what the caller gets.
func mismatchSentences(schemaDocument json.RawMessage, document any, subject string) []string {
	found := map[string]bool{}
	collectMismatches(objectDocument(schemaDocument), document, subject, found)
	sentences := make([]string, 0, len(found))
	for sentence := range found {
		sentences = append(sentences, sentence)
	}
	sort.Strings(sentences)
	return sentences
}

func collectMismatches(schema map[string]any, document any, path string, found map[string]bool) {
	if schema == nil {
		return
	}
	// A branch reached through anyOf is one of several shapes the document may
	// take, so a mismatch against any single branch is not a fault to report.
	if _, isUnion := schema["anyOf"]; isUnion {
		return
	}
	if sentence := typeMismatch(schema, document, path); sentence != "" {
		found[sentence] = true
		return
	}
	if sentence := choiceMismatch(schema, document, path); sentence != "" {
		found[sentence] = true
		return
	}
	if sentence := boundMismatch(schema, document, path); sentence != "" {
		found[sentence] = true
	}
	switch value := document.(type) {
	case map[string]any:
		collectObjectMismatches(schema, value, path, found)
	case []any:
		items, _ := schema["items"].(map[string]any)
		for index, entry := range value {
			collectMismatches(items, entry, path+"["+strconv.Itoa(index)+"]", found)
		}
	}
}

func collectObjectMismatches(schema map[string]any, document map[string]any, path string, found map[string]bool) {
	properties, _ := schema["properties"].(map[string]any)
	for _, name := range requiredNames(schema) {
		if _, isGiven := document[name]; !isGiven {
			found[joinPath(path, name)+" is required and is missing"] = true
		}
	}
	for name, field := range document {
		fieldSchema, isDeclared := properties[name].(map[string]any)
		if !isDeclared {
			if schema["additionalProperties"] == false {
				found[joinPath(path, name)+" is not a field this tool takes"] = true
			}
			continue
		}
		collectMismatches(fieldSchema, field, joinPath(path, name), found)
	}
}

func typeMismatch(schema map[string]any, document any, path string) string {
	wanted, isDeclared := schema["type"].(string)
	if !isDeclared || jsonTypeOf(document) == wanted {
		return ""
	}
	if wanted == "number" && jsonTypeOf(document) == "integer" {
		return ""
	}
	return path + " must be " + article(wanted) + ", and it is " + renderValue(document)
}

func choiceMismatch(schema map[string]any, document any, path string) string {
	choices, hasChoices := schema["enum"].([]any)
	if declared, hasConstant := schema["const"]; hasConstant {
		choices, hasChoices = []any{declared}, true
	}
	if !hasChoices || len(choices) == 0 {
		return ""
	}
	for _, choice := range choices {
		if sameJSONValue(choice, document) {
			return ""
		}
	}
	rendered := make([]string, 0, len(choices))
	for _, choice := range choices {
		rendered = append(rendered, renderValue(choice))
	}
	return path + " must be one of " + strings.Join(rendered, ", ") + ", and it is " + renderValue(document)
}

// Every constraint the catalog puts on a value, said the way the caller would
// have to fix it. A keyword missing here is a fault the walk cannot see, and it
// stays invisible whenever some other fault is found, so
// TestEveryConstraintTheCatalogUsesIsExplained fails when the catalog starts
// using one this does not know.
var explainedKeywords = map[string]bool{
	"additionalProperties": true,
	"anyOf":                true,
	"const":                true,
	"enum":                 true,
	"exclusiveMinimum":     true,
	"items":                true,
	"maxItems":             true,
	"maxLength":            true,
	"maximum":              true,
	"minItems":             true,
	"minLength":            true,
	"minProperties":        true,
	"minimum":              true,
	"pattern":              true,
	"properties":           true,
	"required":             true,
	"type":                 true,
	"uniqueItems":          true,
}

func boundMismatch(schema map[string]any, document any, path string) string {
	switch value := document.(type) {
	case string:
		return textBoundMismatch(schema, value, path)
	case float64:
		return numberBoundMismatch(schema, value, path)
	case []any:
		return listBoundMismatch(schema, value, path)
	case map[string]any:
		if least, hasLeast := countOf(schema, "minProperties"); hasLeast && len(value) < least {
			return path + " must carry at least " + counted(least, "field") + ", and it carries " + counted(len(value), "field")
		}
	}
	return ""
}

func textBoundMismatch(schema map[string]any, value string, path string) string {
	length := len([]rune(value))
	if least, hasLeast := countOf(schema, "minLength"); hasLeast && length < least {
		return path + " must be at least " + counted(least, "character") + " long, and it is " + renderValue(value)
	}
	if most, hasMost := countOf(schema, "maxLength"); hasMost && length > most {
		return path + " must be at most " + counted(most, "character") + " long, and it is " + counted(length, "character")
	}
	expression, hasExpression := schema["pattern"].(string)
	if !hasExpression {
		return ""
	}
	matcher, errorValue := regexp.Compile(expression)
	if errorValue != nil || matcher.MatchString(value) {
		return ""
	}
	return path + " must match " + expression + ", and it is " + renderValue(value)
}

func numberBoundMismatch(schema map[string]any, value float64, path string) string {
	if least, hasLeast := schema["minimum"].(float64); hasLeast && value < least {
		return path + " must be at least " + renderNumber(least) + ", and it is " + renderNumber(value)
	}
	if least, hasLeast := schema["exclusiveMinimum"].(float64); hasLeast && value <= least {
		return path + " must be greater than " + renderNumber(least) + ", and it is " + renderNumber(value)
	}
	if most, hasMost := schema["maximum"].(float64); hasMost && value > most {
		return path + " must be at most " + renderNumber(most) + ", and it is " + renderNumber(value)
	}
	return ""
}

func listBoundMismatch(schema map[string]any, value []any, path string) string {
	if least, hasLeast := countOf(schema, "minItems"); hasLeast && len(value) < least {
		return path + " must carry at least " + counted(least, "entry") + ", and it carries " + counted(len(value), "entry")
	}
	if most, hasMost := countOf(schema, "maxItems"); hasMost && len(value) > most {
		return path + " must carry at most " + counted(most, "entry") + ", and it carries " + counted(len(value), "entry")
	}
	if schema["uniqueItems"] != true {
		return ""
	}
	seen := map[string]bool{}
	for _, entry := range value {
		rendered := renderValue(entry)
		if seen[rendered] {
			return path + " must not repeat an entry, and it repeats " + rendered
		}
		seen[rendered] = true
	}
	return ""
}

func counted(count int, noun string) string {
	if count == 1 {
		return "1 " + noun
	}
	if noun == "entry" {
		return strconv.Itoa(count) + " entries"
	}
	return strconv.Itoa(count) + " " + noun + "s"
}

func countOf(schema map[string]any, keyword string) (int, bool) {
	declared, isDeclared := schema[keyword].(float64)
	return int(declared), isDeclared
}

func renderNumber(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func requiredNames(schema map[string]any) []string {
	declared, _ := schema["required"].([]any)
	names := make([]string, 0, len(declared))
	for _, name := range declared {
		if text, isText := name.(string); isText {
			names = append(names, text)
		}
	}
	sort.Strings(names)
	return names
}

func jsonTypeOf(document any) string {
	switch value := document.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case float64:
		if value == float64(int64(value)) {
			return "integer"
		}
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

func article(wanted string) string {
	switch wanted {
	case "array", "object", "integer":
		return "an " + wanted
	case "null":
		return "null"
	}
	return "a " + wanted
}

const renderedValueLimit = 80

func renderValue(document any) string {
	encoded, errorValue := json.Marshal(document)
	if errorValue != nil {
		return jsonTypeOf(document)
	}
	if len(encoded) <= renderedValueLimit {
		return string(encoded)
	}
	return string(encoded[:renderedValueLimit]) + "…"
}

func sameJSONValue(left any, right any) bool {
	leftEncoded, leftError := json.Marshal(left)
	rightEncoded, rightError := json.Marshal(right)
	if leftError != nil || rightError != nil {
		return false
	}
	return string(leftEncoded) == string(rightEncoded)
}
