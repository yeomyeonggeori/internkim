package admind

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var importerFlagPattern = regexp.MustCompile(`--([a-z][a-z-]*)`)
var importerFlagDefinition = regexp.MustCompile(`flag\.(?:String|Int64|Bool)\("([a-z][a-z-]*)"`)

// The importer moved its secrets to path flags and both callers kept passing
// values, so a fresh binary died on "flag provided but not defined" before it
// read a single post. Nothing caught it because the device was still running a
// binary old enough to accept the old spelling.
func TestEveryFlagTheImportCallersPassIsDefined(t *testing.T) {
	defined := definedImporterFlags(t)
	for name, command := range map[string]string{
		"recovery.go: re-import":        buzzReimportCommand(),
		"recovery.go: refresh profiles": buzzRefreshProfilesCommand(),
	} {
		for _, flagName := range passedImporterFlags(command) {
			if !defined[flagName] {
				t.Errorf("%s passes --%s and buzz-migrate does not define it", name, flagName)
			}
		}
	}
}

// keeperFrom dies on a partial set - three of the four address nothing on their
// own - so a caller that learned only some of them takes the importer down
// before it reads a post.
func TestTheCentralPlaneFlagsArePassedTogetherOrNotAtAll(t *testing.T) {
	for name, command := range map[string]string{
		"recovery.go: re-import":        buzzReimportCommand(),
		"recovery.go: refresh profiles": buzzRefreshProfilesCommand(),
	} {
		if !strings.Contains(command, "$CENTRAL") {
			t.Errorf("%s no longer reaches the central plane, so the company picture cannot be read", name)
			continue
		}
		assignment := ""
		for _, line := range strings.Split(command, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), `CENTRAL="--`) {
				assignment = line
			}
		}
		if assignment == "" {
			t.Errorf("%s uses $CENTRAL without ever setting it to flags", name)
			continue
		}
		for _, flagName := range []string{"--app-url", "--agent-key-path", "--supabase-url", "--supabase-publishable-key"} {
			if !strings.Contains(assignment, flagName) {
				t.Errorf("%s sets $CENTRAL without %s, and a partial set kills the importer", name, flagName)
			}
		}
	}
}

func definedImporterFlags(t *testing.T) map[string]bool {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "cmd", "buzz-migrate", "main.go"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defined := map[string]bool{}
	for _, match := range importerFlagDefinition.FindAllStringSubmatch(string(document), -1) {
		defined[match[1]] = true
	}
	if len(defined) == 0 {
		t.Fatal("buzz-migrate defines no flags, so this guard is reading the wrong file")
	}
	return defined
}

// The invocation is one shell command spread over continued lines, and the
// script around it runs others that carry flags of their own.
func passedImporterFlags(command string) []string {
	names := []string{}
	inInvocation := false
	for _, line := range strings.Split(command, "\n") {
		if strings.Contains(line, "buzz-migrate") && strings.HasSuffix(strings.TrimSpace(line), "\\") {
			inInvocation = true
			continue
		}
		if !inInvocation {
			continue
		}
		for _, match := range importerFlagPattern.FindAllStringSubmatch(line, -1) {
			names = append(names, match[1])
		}
		if !strings.HasSuffix(strings.TrimSpace(line), "\\") {
			inInvocation = false
		}
	}
	return names
}
