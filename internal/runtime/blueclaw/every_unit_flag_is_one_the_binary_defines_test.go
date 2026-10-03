package blueclaw

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
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
		"buzz-media":      BuzzMediaServiceUnit(),
		"internkim-relay": RelayServiceUnit(),
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

func TestEveryPlaneLauncherFlagIsOneTheBinaryDefines(t *testing.T) {
	launcherPath := filepath.Join("..", "..", "..", "web", "tests", "plane", "a-company-plane.ts")
	launcherSource, errorValue := os.ReadFile(launcherPath)
	if errorValue != nil {
		t.Fatalf("read %s: %v", launcherPath, errorValue)
	}
	commandDirectoryByLauncherName := map[string]string{
		"capabilityd": "internkim-capabilityd",
		"admind":      "internkim-admind",
	}

	checkedFlagCount := 0
	for launcherName, commandDirectory := range commandDirectoryByLauncherName {
		definedFlags := flagNamesDefinedBy(t, commandDirectory)
		for _, flagName := range flagNamesPassedByLauncher(t, string(launcherSource), launcherName) {
			if !definedFlags[flagName] {
				t.Fatalf("the plane starts %s with -%s, which it does not define; "+
					"the flag package exits 2 and the plane never comes up", commandDirectory, flagName)
			}
			checkedFlagCount++
		}
	}
	if checkedFlagCount == 0 {
		t.Fatal("no plane launcher flag was checked, so this test is reading the wrong place")
	}
}

func flagNamesPassedByLauncher(t *testing.T, launcherSource string, launcherName string) []string {
	t.Helper()
	functionPattern := regexp.MustCompile(`(?s)export function ` + launcherName + `ArgumentsForPlane\(.*?\n}\n`)
	functionSource := functionPattern.FindString(launcherSource)
	if functionSource == "" {
		t.Fatalf("a-company-plane.ts no longer has %sArgumentsForPlane, so this test is reading the wrong place", launcherName)
	}
	flagNames := []string{}
	for _, match := range regexp.MustCompile(`'-{1,2}([a-z][a-z0-9-]*)':`).FindAllStringSubmatch(functionSource, -1) {
		flagNames = append(flagNames, match[1])
	}
	return flagNames
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
