package capabilityprotocol

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const TaskEventConfirmationRequested = "confirmation.requested"

type generatedTaskEventNameSchema struct {
	AnyOf []struct {
		Enum    []string `json:"enum"`
		Pattern string   `json:"pattern"`
	} `json:"anyOf"`
}

type generatedToolTaskEventSuffixSchema struct {
	Enum []string `json:"enum"`
}

var (
	declaredTaskEventNames        = mustLoadDeclaredTaskEventNames()
	toolTaskEventNameMatch        = mustCompileToolTaskEventNamePattern()
	declaredToolTaskEventSuffixes = mustLoadToolTaskEventSuffixes()
)

func DeclaredTaskEventNames() []string {
	names := make([]string, 0, len(declaredTaskEventNames))
	for name := range declaredTaskEventNames {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func DeclaredToolTaskEventSuffixes() []string {
	return append([]string{}, declaredToolTaskEventSuffixes...)
}

func IsDeclaredTaskEventName(name string) bool {
	trimmedName := strings.TrimSpace(name)
	if declaredTaskEventNames[trimmedName] {
		return true
	}
	return toolTaskEventNameMatch.MatchString(trimmedName)
}

func ToolTaskEventToolName(name string) (string, bool) {
	trimmedName := strings.TrimSpace(name)
	if !toolTaskEventNameMatch.MatchString(trimmedName) {
		return "", false
	}
	for _, suffix := range declaredToolTaskEventSuffixes {
		if !strings.HasSuffix(trimmedName, suffix) {
			continue
		}
		afterPrefix := trimmedName[strings.Index(trimmedName, ".")+1:]
		return strings.TrimSuffix(afterPrefix, suffix), true
	}
	return "", false
}

func mustLoadDeclaredTaskEventNames() map[string]bool {
	names := map[string]bool{}
	for _, branch := range mustReadTaskEventNameSchema().AnyOf {
		for _, name := range branch.Enum {
			names[name] = true
		}
	}
	if len(names) == 0 {
		panic(fmt.Errorf("the generated task-event-name schema declares no fixed names"))
	}
	return names
}

func mustCompileToolTaskEventNamePattern() *regexp.Regexp {
	for _, branch := range mustReadTaskEventNameSchema().AnyOf {
		if branch.Pattern == "" {
			continue
		}
		matcher, errorValue := regexp.Compile(branch.Pattern)
		if errorValue != nil {
			panic(errorValue)
		}
		return matcher
	}
	panic(fmt.Errorf("the generated task-event-name schema declares no tool event pattern"))
}

func mustReadTaskEventNameSchema() generatedTaskEventNameSchema {
	schema := generatedTaskEventNameSchema{}
	mustReadGeneratedSchema("task-event-name", &schema)
	return schema
}

func mustLoadToolTaskEventSuffixes() []string {
	schema := generatedToolTaskEventSuffixSchema{}
	mustReadGeneratedSchema("tool-task-event-suffix", &schema)
	if len(schema.Enum) == 0 {
		panic(fmt.Errorf("the generated tool-task-event-suffix schema declares no suffixes"))
	}
	return schema.Enum
}

func mustReadGeneratedSchema(schemaName string, document any) {
	source, errorValue := generatedCatalogFiles.ReadFile("generated/json-schema/" + schemaName + ".schema.json")
	if errorValue != nil {
		panic(errorValue)
	}
	if errorValue := json.Unmarshal(source, document); errorValue != nil {
		panic(errorValue)
	}
}
