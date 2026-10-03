package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInDirectory(t *testing.T, directory string, arguments ...string) string {
	t.Helper()
	identity := []string{"-c", "user.name=Sample", "-c", "user.email=sample@example.com", "-c", "protocol.file.allow=always"}
	output, errorValue := exec.Command("git", append(append([]string{"-C", directory}, identity...), arguments...)...).CombinedOutput()
	if errorValue != nil {
		t.Fatalf("git %v in %s: %s", arguments, directory, output)
	}
	return strings.TrimSpace(string(output))
}

func commitFile(t *testing.T, directory string, relativePath string, content string) {
	t.Helper()
	writeFileForTest(t, filepath.Join(directory, relativePath), content)
	gitInDirectory(t, directory, "add", relativePath)
	gitInDirectory(t, directory, "commit", "-q", "-m", "change "+relativePath)
}

func cloneWithOrigin(t *testing.T, destination string) (string, string) {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	gitInDirectory(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", origin)
	gitInDirectory(t, filepath.Dir(destination), "clone", "-q", origin, destination)
	gitInDirectory(t, destination, "checkout", "-q", "-b", "main")
	commitFile(t, destination, "first.txt", "first")
	gitInDirectory(t, destination, "push", "-q", "origin", "main")
	return destination, origin
}

func pushFromAnotherClone(t *testing.T, origin string) {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	gitInDirectory(t, filepath.Dir(other), "clone", "-q", origin, other)
	commitFile(t, other, "moved-on.txt", "moved on")
	gitInDirectory(t, other, "push", "-q", "origin", "main")
}

type shippableTree struct {
	root           string
	rootOrigin     string
	blueclaw       string
	blueclawOrigin string
}

func newShippableTree(t *testing.T) shippableTree {
	t.Helper()
	parent := t.TempDir()
	root, rootOrigin := cloneWithOrigin(t, filepath.Join(parent, "root"))
	blueclawPath := filepath.Join(root, ".dependency", "blueclaw")
	if errorValue := os.MkdirAll(filepath.Dir(blueclawPath), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	_, blueclawOrigin := cloneWithOrigin(t, blueclawPath)
	commitFile(t, root, "web/page.txt", "page")
	gitInDirectory(t, root, "add", ".dependency/blueclaw")
	gitInDirectory(t, root, "commit", "-q", "-m", "record blueclaw")
	gitInDirectory(t, root, "push", "-q", "origin", "main")
	return shippableTree{root: root, rootOrigin: rootOrigin, blueclaw: blueclawPath, blueclawOrigin: blueclawOrigin}
}

func writeFileForTest(t *testing.T, path string, content string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(content), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
}
