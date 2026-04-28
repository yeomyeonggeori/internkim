package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentBrowserSkillInstallScriptPrefersInstalledCliSkill(t *testing.T) {
	script := agentBrowserSkillInstallScript("/tmp/fallback.md", "fallback skill")
	requiredFragments := []string{
		"/root/.blueclaw/workspace/.agents/skills/agent-browser",
		"agent-browser skills get core --full",
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

func TestFirstbootInstallsAgentBrowserSkill(t *testing.T) {
	script := renderFirstbootToolsSection()
	requiredFragments := []string{
		"agent-browser install",
		"agent-browser skills get core --full",
		"$STAGE/agent-browser-skill/SKILL.md",
		"/root/.blueclaw/workspace/.agents/skills/agent-browser",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected firstboot tools section to include %q", fragment)
		}
	}
}
