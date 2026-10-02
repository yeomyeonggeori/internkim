package blueclaw

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const fewestFlagsAnInternKimBinaryDefines = 5

func TestEveryUnitFlagIsOneTheBinaryDefines(t *testing.T) {
	commandDirectoryByBinaryPath := map[string]string{
		CapabilitydBinaryPath: "internkim-capabilityd",
		AdmindBinaryPath:      "internkim-admind",
		LinuxCompanyHostLayout().BinaryPath(CapabilitydName): "internkim-capabilityd",
		LinuxCompanyHostLayout().BinaryPath(AdmindName):      "internkim-admind",
	}

	units := map[string]string{
		"blueclaw":                     BlueclawServiceUnit(),
		"internkim-capabilityd":        CapabilitydServiceUnit(),
		"internkim-capabilityd-remote": CapabilitydServiceUnitForLocalInferenceMode("remote"),
		"internkim-admind":             AdmindServiceUnit(),
		"buzz-relay":                   BuzzRelayServiceUnit("wss://relay.example.test"),
		"chatd":                        ChatdServiceUnit("wss://relay.example.test"),
		"buzz-media":                   BuzzMediaServiceUnit(),
		"llama-cpp":                    LlamaCppServiceUnit(),
		"llama-cpp-embedding":          LlamaCppEmbeddingServiceUnit(),
		"internkim-relay":              RelayServiceUnit(),
		"internkim-users-sync":         InternKimUsersSyncServiceUnit(),
	}
	for _, unit := range CompanyPackageUnits() {
		units["packaged "+unit.Name] = unit.Contents
	}

	checkedFlagCount := 0
	for unitName, unitDocument := range units {
		for _, startLine := range execStartLines(unitDocument) {
			binaryPath, flagNames := binaryAndFlagsInvokedBy(startLine)
			commandDirectory, isOurs := commandDirectoryByBinaryPath[binaryPath]
			if !isOurs {
				continue
			}
			definedFlags := flagNamesDefinedBy(t, commandDirectory)
			for _, flagName := range flagNames {
				if !definedFlags[flagName] {
					t.Fatalf("the %s unit starts %s with -%s, which it does not define; "+
						"the flag package exits 2 and systemd restarts the crash forever", unitName, binaryPath, flagName)
				}
				checkedFlagCount++
			}
		}
	}
	if checkedFlagCount == 0 {
		t.Fatal("no unit flag was checked, so this test is reading the wrong place")
	}
}

func execStartLines(unitDocument string) []string {
	lines := []string{}
	for _, line := range strings.Split(unitDocument, "\n") {
		if strings.HasPrefix(line, "ExecStart=") {
			lines = append(lines, strings.TrimPrefix(line, "ExecStart="))
		}
	}
	return lines
}

func binaryAndFlagsInvokedBy(startLine string) (string, []string) {
	fields := strings.Fields(startLine)
	if len(fields) == 0 {
		return "", nil
	}
	flagNames := []string{}
	for _, field := range fields[1:] {
		if !strings.HasPrefix(field, "-") {
			continue
		}
		flagName := strings.TrimLeft(field, "-")
		flagName, _, _ = strings.Cut(flagName, "=")
		if flagName != "" {
			flagNames = append(flagNames, flagName)
		}
	}
	return fields[0], flagNames
}

func flagNamesDefinedBy(t *testing.T, commandDirectory string) map[string]bool {
	t.Helper()
	sourcePath := filepath.Join("..", "..", "..", "cmd", commandDirectory, "main.go")
	fileSet := token.NewFileSet()
	sourceFile, errorValue := parser.ParseFile(fileSet, sourcePath, nil, 0)
	if errorValue != nil {
		t.Fatalf("read %s: %v", sourcePath, errorValue)
	}
	definedFlags := map[string]bool{}
	ast.Inspect(sourceFile, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		packageName, isIdentifier := selector.X.(*ast.Ident)
		if !isIdentifier || packageName.Name != "flag" {
			return true
		}
		for _, argument := range call.Args {
			literal, isLiteral := argument.(*ast.BasicLit)
			if !isLiteral || literal.Kind != token.STRING {
				continue
			}
			flagName, errorValue := strconv.Unquote(literal.Value)
			if errorValue == nil && flagName != "" {
				definedFlags[flagName] = true
			}
			break
		}
		return true
	})
	if len(definedFlags) < fewestFlagsAnInternKimBinaryDefines {
		t.Fatalf("read only %d flags from %s, so this test is reading the wrong place", len(definedFlags), sourcePath)
	}
	return definedFlags
}
