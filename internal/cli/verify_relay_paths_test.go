package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var relayWorkspacePathPattern = regexp.MustCompile(`'person\.[a-z.]+':\s*'([^']+)'`)

func relayWorkspacePaths(t *testing.T) []string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.Join("..", "..", "host", "relay", "relay.ts"))
	if errorValue != nil {
		t.Fatalf("read the relay source: %v", errorValue)
	}
	matches := relayWorkspacePathPattern.FindAllStringSubmatch(string(source), -1)
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		paths = append(paths, match[1])
	}
	return paths
}

func TestVerifyAPIScriptProbesEveryPathTheRelayForwards(t *testing.T) {
	paths := relayWorkspacePaths(t)
	if len(paths) == 0 {
		t.Fatal("the relay forwards no workspace paths, so this guard is reading the wrong source")
	}

	script := verifyAPIScript()
	for _, path := range paths {
		if !strings.Contains(script, path) {
			t.Errorf("the relay forwards %s but device verification never asks for it, so a device that cannot answer it ships unnoticed", path)
		}
	}
}

func TestVerifyAPIScriptFailsOnARefusedRelayPath(t *testing.T) {
	script := verifyAPIScript()
	if !strings.Contains(script, "403|404)") {
		t.Error("device verification must fail on a refused relay path; a 403 is what a device missing the loopback actor answers")
	}
	if !strings.Contains(script, "X-INTERNKIM-REQUESTER-EMAIL") {
		t.Error("the relay names its requester in a header, so verification has to ask the same way it does")
	}
}
