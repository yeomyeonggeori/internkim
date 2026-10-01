package blueclaw_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func theEntrypointAsParsed(t *testing.T) *syntax.File {
	t.Helper()
	path := filepath.Join(repositoryRootFromHere, "host", "entrypoint.sh")
	source, openError := os.Open(path)
	if openError != nil {
		t.Fatal(openError)
	}
	defer source.Close()
	script, parseError := syntax.NewParser().Parse(source, path)
	if parseError != nil {
		t.Fatalf("host/entrypoint.sh does not parse as a shell script: %v", parseError)
	}
	return script
}

func theFunctionsTheEntrypointDefines(script *syntax.File) map[string]bool {
	defined := map[string]bool{}
	syntax.Walk(script, func(node syntax.Node) bool {
		if declaration, isDeclaration := node.(*syntax.FuncDecl); isDeclaration {
			defined[declaration.Name.Value] = true
		}
		return true
	})
	return defined
}

func theProgramsTheEntrypointRuns(t *testing.T) []string {
	t.Helper()
	script := theEntrypointAsParsed(t)
	definedHere := theFunctionsTheEntrypointDefines(script)
	invoked := map[string]bool{}
	printer := syntax.NewPrinter()
	syntax.Walk(script, func(node syntax.Node) bool {
		call, isCall := node.(*syntax.CallExpr)
		if !isCall || len(call.Args) == 0 {
			return true
		}
		name := call.Args[0].Lit()
		if name == "" {
			written := strings.Builder{}
			if printError := printer.Print(&written, call.Args[0]); printError != nil {
				t.Fatal(printError)
			}
			t.Errorf("host/entrypoint.sh runs %s, whose name is decided at run time, so no reader can say "+
				"which program that is; give it a literal name or this check cannot cover it", written.String())
			return true
		}
		if definedHere[name] || interp.IsBuiltin(name) {
			return true
		}
		invoked[name] = true
		return true
	})
	programs := make([]string, 0, len(invoked))
	for program := range invoked {
		programs = append(programs, program)
	}
	sort.Strings(programs)
	return programs
}

func TestEveryProgramTheEntrypointRunsIsOneTheDeclarationNames(t *testing.T) {
	programs := theProgramsTheEntrypointRuns(t)
	if len(programs) == 0 || !slicesContain(programs, blueclaw.BlueclawName) {
		t.Fatalf("this reader found %v in host/entrypoint.sh, which does not include the agent the script exists "+
			"to start, so it is reading something other than the script", programs)
	}

	declared := map[string]bool{}
	for _, program := range append(blueclaw.ProgramsTheHostEntrypointRuns(), blueclaw.ProgramsTheBundledSkillsRun()...) {
		declared[program] = true
	}
	undeclared := []string{}
	for _, program := range programs {
		if !declared[program] {
			undeclared = append(undeclared, program)
		}
	}
	if len(undeclared) == 0 {
		return
	}
	t.Fatalf("host/entrypoint.sh runs %s and internal/runtime/blueclaw names none of them, so a box built from "+
		"the declaration passes --check-programs, comes up, and dies at the line that needs one",
		strings.Join(undeclared, ", "))
}

func slicesContain(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
