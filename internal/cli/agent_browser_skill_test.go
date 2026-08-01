package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
)

func TestAgentBrowserSkillInstallScriptUsesVendoredWorkspaceSkill(t *testing.T) {
	script := agentBrowserSkillInstallScript("/tmp/fallback.md", "fallback skill")
	requiredFragments := []string{
		"/root/.blueclaw/workspace/.agents/skills/agent-browser",
		"/tmp/fallback.md",
		"fallback skill",
		"chown -R blueclaw:blueclaw /root/.blueclaw/workspace/.agents",
		"chown -R root:root \"$skillDir\"",
		"chmod -R a+rX,go-w \"$skillDir\"",
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
	for _, fragment := range []string{
		"agent-browser",
		"browser_open",
		"browser_snapshot",
		"interactive fallback",
		"user input such as login/MFA/captcha",
	} {
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

	for _, fragment := range []string{
		"## Retrieval And Browser",
		"`browser_snapshot`",
		"interactive fallback",
		"Use `web_fetch` for ordinary public URL lookup",
		"## File Delivery",
		"## Memory",
		"## Approval Handling",
		"## Honesty about Tool Failures",
	} {
		if !strings.Contains(agentsMarkdown, fragment) {
			t.Fatalf("expected board agents markdown to include %q", fragment)
		}
	}
}

func TestLoadWorkspaceDocumentsUsesBoardAssets(t *testing.T) {
	scriptDir, errorValue := filepath.Abs("../..")
	if errorValue != nil {
		t.Fatalf("expected script dir: %v", errorValue)
	}

	documents, errorValue := loadWorkspaceDocuments(scriptDir)
	if errorValue != nil {
		t.Fatalf("expected workspace documents: %v", errorValue)
	}
	for _, expectation := range []struct {
		name     string
		document string
		fragment string
	}{
		{name: "identity", document: documents.Identity, fragment: "runtime bot profile"},
		{name: "identity name policy", document: documents.Identity, fragment: "Treat names as a single field"},
		{name: "soul", document: documents.Soul, fragment: "SOUL.md"},
		{name: "bot profile", document: documents.BotProfile, fragment: `displayName: 김인턴`},
	} {
		if !strings.Contains(expectation.document, expectation.fragment) {
			t.Fatalf("expected %s document to include %q", expectation.name, expectation.fragment)
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
		"install_device_browser_runtime",
		"/usr/local/bin/lightpanda",
		"$STAGE/.agents/skills/agent-browser/SKILL.md",
		"$STAGE/tools",
		"/root/.blueclaw/workspace/.agents/skills/agent-browser",
		"/root/.blueclaw/workspace/tools",
		"chown -R root:root \"$agentBrowserSkillDir\"",
		"chmod -R a+rX,go-w /root/.blueclaw/workspace/skills",
		"chmod -R a+rX,go-w /root/.blueclaw/workspace/tools",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected firstboot tools section to include %q", fragment)
		}
	}
	for _, forbiddenFragment := range []string{"agent-browser install", "agent-browser skills get core --full", "chromium-browser", "--engine lightpanda", "lightpanda' --version"} {
		if strings.Contains(script, forbiddenFragment) {
			t.Fatalf("firstboot tools section must not include %q", forbiddenFragment)
		}
	}
}

func TestFirstbootChecksLightpandaVersionSubcommand(t *testing.T) {
	script := renderFirstbootToolsSection()
	expectedCommand := deviceBrowserVersionShellCommand(browserruntime.DeviceBrowserExecutablePath)
	if !strings.Contains(script, expectedCommand) {
		t.Fatalf("expected firstboot tools section to include %q", expectedCommand)
	}
}

func TestFirstbootUsesCanonicalConfigurationPaths(t *testing.T) {
	script := renderFirstbootStagingSection()

	for _, fragment := range []string{
		"/root/.internkim/config/admin-email",
		"/root/.blueclaw/workspace/.blueclaw/runtime/current/migrations",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected firstboot staged files section to include %q", fragment)
		}
	}
	for _, forbiddenFragment := range []string{
		"/root/.internkim/admin-email",
		"/root/.blueclaw/runtime/current/migrations",
	} {
		if strings.Contains(script, forbiddenFragment) {
			t.Fatalf("firstboot staged files section must not include %q", forbiddenFragment)
		}
	}
}
