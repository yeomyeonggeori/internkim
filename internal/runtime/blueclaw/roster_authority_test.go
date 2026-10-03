package blueclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const rosterAuthorityRule = "the host writes the roster and the agent never authors its own. " +
	"Change the roster by editing the delivered policy document and calling " +
	"deliverBlueclawPolicy in admind; never by asking the agent to mutate its own people."

const agentRosterMutationEndpoint = "/admin/api/people"

func servicesTheHostRunsUnattended() []string {
	return []string{"cmd", "internal/admind", "internal/runtime"}
}

func TestServicesTheHostRunsUnattendedNeverAskTheAgentToWriteTheRoster(t *testing.T) {
	repositoryRoot := repositoryRootPath(t)
	for _, directory := range servicesTheHostRunsUnattended() {
		for _, sourcePath := range goSourcePaths(t, filepath.Join(repositoryRoot, directory)) {
			content, readError := os.ReadFile(sourcePath)
			if readError != nil {
				t.Fatalf("read %s: %v", sourcePath, readError)
			}
			if !strings.Contains(string(content), agentRosterMutationEndpoint) {
				continue
			}
			relativePath, _ := filepath.Rel(repositoryRoot, sourcePath)
			t.Errorf("%s names the agent's %s endpoint: %s", relativePath, agentRosterMutationEndpoint, rosterAuthorityRule)
		}
	}
}

func goSourcePaths(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	walkError := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkError error) error {
		if walkError != nil {
			return walkError
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if walkError != nil {
		t.Fatalf("walk %s: %v", root, walkError)
	}
	return paths
}

func repositoryRootPath(t *testing.T) string {
	t.Helper()
	directory, errorValue := os.Getwd()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for {
		if _, statError := os.Stat(filepath.Join(directory, "go.mod")); statError == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("no go.mod above the test working directory")
		}
		directory = parent
	}
}
