package blueclawworkspace

import (
	"os"
	"path/filepath"
)

func AssetsPath(scriptDir string) string {
	return filepath.Join(scriptDir, "assets", "blueclaw-workspace")
}

func SkillsPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "skills")
}

func AgentSkillsPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), ".agents", "skills")
}

func AgentBrowserSkillPath(scriptDir string) string {
	return filepath.Join(AgentSkillsPath(scriptDir), "agent-browser", "SKILL.md")
}

func AgentsPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "AGENTS.md")
}

func IdentityPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "IDENTITY.md")
}

func SoulPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "SOUL.md")
}

func BotProfilePath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "BOT_PROFILE.yaml")
}

func GasSourcePath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "gas", "Code.gs")
}

func ReadGasBridgeCode(scriptDir string) (string, error) {
	data, error := os.ReadFile(GasSourcePath(scriptDir))
	if error != nil {
		return "", error
	}
	return string(data), nil
}
