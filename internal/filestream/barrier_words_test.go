package filestream

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestTheAgentAsksTheBarrierInTheWordsItAnswers(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	agentClient := filepath.Join(filepath.Dir(thisFile), "..", "..", ".dependency", "blueclaw", "internal", "filejournal", "barrier.go")
	parsed, errorValue := parser.ParseFile(token.NewFileSet(), agentClient, nil, 0)
	if errorValue != nil {
		t.Fatalf("reading the agent's barrier client at %s: %v", agentClient, errorValue)
	}
	agentWords := map[string]string{}
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, isValue := node.(*ast.ValueSpec)
		if !isValue || len(spec.Values) != 1 {
			return true
		}
		literal, isLiteral := spec.Values[0].(*ast.BasicLit)
		if isLiteral && literal.Kind == token.STRING {
			agentWords[spec.Names[0].Name], _ = strconv.Unquote(literal.Value)
		}
		return true
	})
	for name, ours := range map[string]string{"barrierRequest": barrierRequest, "barrierKept": barrierKept, "barrierUnjournaled": barrierUnjournaled} {
		if agentWords[name] != ours {
			t.Errorf("the agent's %s is %q and the barrier's is %q", name, agentWords[name], ours)
		}
	}
}
