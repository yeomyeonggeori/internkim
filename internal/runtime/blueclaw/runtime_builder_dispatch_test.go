package blueclaw

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var shellFunctionDefinition = regexp.MustCompile(`(?m)^([a-z_][a-z0-9_]*)\(\) \{$`)

func TestPrepareRuntimeScriptDefinesEveryFunctionTheBuilderDispatchReaches(t *testing.T) {
	lines := prepareRuntimeScriptLines(t)
	dispatchLine := indexOfScriptLine(t, lines, `case "$builder" in`)
	definitionLines := shellFunctionDefinitionLines(lines)

	for _, functionName := range reachableShellFunctions(definitionLines, lines, "run_container_builder", "run_ssh_builder") {
		definitionLine, isDefined := definitionLines[functionName]
		if !isDefined {
			t.Fatalf("the builder dispatch reaches %s, which the script never defines", functionName)
		}
		if definitionLine > dispatchLine {
			t.Fatalf(
				"the builder dispatch on line %d reaches %s, defined on line %d; bash has not read that definition yet",
				dispatchLine+1, functionName, definitionLine+1,
			)
		}
	}
}

func prepareRuntimeScriptLines(t *testing.T) []string {
	t.Helper()
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	return strings.Split(string(document), "\n")
}

func indexOfScriptLine(t *testing.T, lines []string, wanted string) int {
	t.Helper()
	for index, line := range lines {
		if strings.TrimSpace(line) == wanted {
			return index
		}
	}
	t.Fatalf("expected the prepare script to contain %q", wanted)
	return 0
}

func shellFunctionDefinitionLines(lines []string) map[string]int {
	definitionLines := map[string]int{}
	for index, line := range lines {
		match := shellFunctionDefinition.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		definitionLines[match[1]] = index
	}
	return definitionLines
}

func shellFunctionBody(definitionLines map[string]int, lines []string, functionName string) []string {
	start, isDefined := definitionLines[functionName]
	if !isDefined {
		return nil
	}
	for end := start + 1; end < len(lines); end++ {
		if lines[end] == "}" {
			return lines[start+1 : end]
		}
	}
	return lines[start+1:]
}

func reachableShellFunctions(definitionLines map[string]int, lines []string, entryPoints ...string) []string {
	visited := map[string]bool{}
	pending := append([]string{}, entryPoints...)
	for len(pending) > 0 {
		functionName := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if visited[functionName] {
			continue
		}
		visited[functionName] = true
		body := strings.Join(shellFunctionBody(definitionLines, lines, functionName), "\n")
		for calledName := range definitionLines {
			if calledName == functionName || visited[calledName] {
				continue
			}
			if shellCallsFunction(body, calledName) {
				pending = append(pending, calledName)
			}
		}
	}
	reachable := make([]string, 0, len(visited))
	for functionName := range visited {
		reachable = append(reachable, functionName)
	}
	sort.Strings(reachable)
	return reachable
}

func shellCallsFunction(body string, functionName string) bool {
	callPattern := regexp.MustCompile(`(^|[^a-zA-Z0-9_$-])` + regexp.QuoteMeta(functionName) + `($|[^a-zA-Z0-9_-])`)
	return callPattern.MatchString(body)
}
