package admind

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnlyARequestStartsABackup(t *testing.T) {
	callers := callersOf(t, "createPlainBackup")
	if len(callers) != 1 || callers[0] != "runBackupJob" {
		t.Fatalf("a backup is written only for an administrator who asked for one; createPlainBackup is called from %v", callers)
	}
	callers = callersOf(t, "runBackupJob")
	if len(callers) != 1 || callers[0] != "createBackup" {
		t.Fatalf("runBackupJob is started only by the backup request handler; it is called from %v", callers)
	}
}

func TestAbandonedBackupIntermediatesAreRemovedAndNothingElse(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{".internkim-backup-20261001T135616Z.tar.gz", "buzz-20260826T140153Z.sql", "attendance-before-overnight-fix.sqlite"} {
		if errorValue := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	removeAbandonedBackupIntermediates(directory)
	entries, errorValue := os.ReadDir(directory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	remaining := []string{}
	for _, entry := range entries {
		remaining = append(remaining, entry.Name())
	}
	if strings.Join(remaining, ",") != "attendance-before-overnight-fix.sqlite,buzz-20260826T140153Z.sql" {
		t.Fatalf("remaining = %v", remaining)
	}
}

func callersOf(t *testing.T, calleeName string) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	packages, errorValue := parser.ParseDir(fileSet, ".", func(information os.FileInfo) bool {
		return !strings.HasSuffix(information.Name(), "_test.go")
	}, 0)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	callers := []string{}
	for _, parsedPackage := range packages {
		for _, file := range parsedPackage.Files {
			for _, declaration := range file.Decls {
				function, isFunction := declaration.(*ast.FuncDecl)
				if !isFunction || function.Body == nil {
					continue
				}
				if callsSelector(function.Body, calleeName) {
					callers = append(callers, function.Name.Name)
				}
			}
		}
	}
	return callers
}

func callsSelector(body *ast.BlockStmt, calleeName string) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		if selector, isSelector := call.Fun.(*ast.SelectorExpr); isSelector && selector.Sel.Name == calleeName {
			found = true
		}
		return !found
	})
	return found
}
