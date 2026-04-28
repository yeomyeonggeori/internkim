package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentBrowserSkillInstallScriptPrefersInstalledCliSkill(t *testing.T) {
	script := agentBrowserSkillInstallScript("/tmp/fallback.md", "fallback skill")
	requiredFragments := []string{
		"/root/.blueclaw/workspace/.agents/skills/agent-browser",
		"agent-browser skills get core --full",
		"https://raw.githubusercontent.com/vercel-labs/agent-browser/refs/heads/main/skill-data/core/SKILL.md",
		"https://raw.githubusercontent.com/vercel-labs/agent-browser/refs/heads/main/skills/agent-browser/SKILL.md",
		"/tmp/fallback.md",
		"fallback skill",
		"chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.agents",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected install script to include %q", fragment)
		}
	}
}

func TestLoadAgentBrowserSkillMarkdownUsesVendoredFallback(t *testing.T) {
	scriptDir, errorValue := filepath.Abs("../..")
	if errorValue != nil {
		t.Fatalf("expected script dir: %v", errorValue)
	}

	skillMarkdown, errorValue := loadAgentBrowserSkillMarkdown(scriptDir)
	if errorValue != nil {
		t.Fatalf("expected vendored skill markdown: %v", errorValue)
	}
	for _, fragment := range []string{"agent-browser", "agent-browser skills get core --full"} {
		if !strings.Contains(skillMarkdown, fragment) {
			t.Fatalf("expected skill markdown to include %q", fragment)
		}
	}
}

func TestLoadWorkspaceAgentsMarkdownUsesBoardAsset(t *testing.T) {
	scriptDir, errorValue := filepath.Abs("../..")
	if errorValue != nil {
		t.Fatalf("expected script dir: %v", errorValue)
	}

	agentsMarkdown, errorValue := loadWorkspaceAgentsMarkdown(scriptDir)
	if errorValue != nil {
		t.Fatalf("expected board agents markdown: %v", errorValue)
	}

	for _, fragment := range []string{"## Browser Automation", "`agent-browser snapshot -i`", "## File Sharing", "## Memory", "## Honesty about Tool Failures"} {
		if !strings.Contains(agentsMarkdown, fragment) {
			t.Fatalf("expected board agents markdown to include %q", fragment)
		}
	}
}

func TestLoadWorkspaceAgentsMarkdownDoesNotFallbackToRepositoryRoot(t *testing.T) {
	scriptDir := t.TempDir()
	if errorValue := os.WriteFile(filepath.Join(scriptDir, "AGENTS.md"), []byte("repository root agents"), 0o644); errorValue != nil {
		t.Fatalf("expected repository agents fixture: %v", errorValue)
	}

	if _, errorValue := loadWorkspaceAgentsMarkdown(scriptDir); errorValue == nil {
		t.Fatalf("expected missing board agents markdown to fail")
	}
}

func TestFirstbootInstallsAgentBrowserSkill(t *testing.T) {
	script := renderFirstbootToolsSection()
	requiredFragments := []string{
		"agent-browser install",
		"agent-browser skills get core --full",
		"skill-data/core/SKILL.md",
		"skills/agent-browser/SKILL.md",
		"$STAGE/agent-browser-skill/SKILL.md",
		"/root/.blueclaw/workspace/.agents/skills/agent-browser",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected firstboot tools section to include %q", fragment)
		}
	}
}
