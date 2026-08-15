package blueclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryDaemonDependenciesArePinned(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, BlueclawSubmodulePath, "tools", "graphiti_memoryd", "requirements.txt"))
	if errorValue != nil {
		t.Fatalf("expected the memory daemon requirements: %v", errorValue)
	}

	for _, line := range strings.Split(string(document), "\n") {
		requirement := strings.TrimSpace(line)
		if requirement == "" || strings.HasPrefix(requirement, "#") {
			continue
		}
		if !strings.Contains(requirement, "==") {
			t.Fatalf("every rootfs build resolves %q afresh; pin it so a guest image is reproducible", requirement)
		}
	}

	if !strings.Contains(string(document), "httpx") {
		t.Fatal("graphiti_core imports httpx without declaring it, so this must declare it or the daemon dies on import")
	}
}

func TestPrepareRuntimeScriptProvesTheMemoryVenvImports(t *testing.T) {
	repositoryRootPath := runtimeArtifactRepositoryRoot(t)
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatalf("expected prepare script: %v", errorValue)
	}
	script := string(document)

	if !strings.Contains(script, "graphiti-venv/bin/python -c") {
		t.Fatal("the graphiti venv ships unproven; a missing import surfaces on a device instead of failing the build")
	}
	if !strings.Contains(script, "LLMClient.set_tracer") {
		t.Fatal("Graphiti calls set_tracer on the client, so the build must prove the installed version still offers it")
	}
}
