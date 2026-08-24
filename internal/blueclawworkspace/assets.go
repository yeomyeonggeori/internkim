package blueclawworkspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

func AssetsPath(scriptDir string) string {
	return filepath.Join(scriptDir, "assets", "blueclaw-workspace")
}

func SkillsPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "skills")
}

func PluginPath(scriptDir string) string {
	return filepath.Join(scriptDir, ".dependency", "internkim-plugin")
}

func PluginSkillsPath(scriptDir string) string {
	return filepath.Join(PluginPath(scriptDir), "skills")
}

func SkillRootPaths(scriptDir string) []string {
	return []string{SkillsPath(scriptDir), PluginSkillsPath(scriptDir)}
}

type SkillDirectory struct {
	Name string
	Path string
}

func SkillDirectories(scriptDir string) ([]SkillDirectory, error) {
	skillDirectories := []SkillDirectory{}
	pathByName := map[string]string{}
	for _, rootPath := range SkillRootPaths(scriptDir) {
		entries, errorValue := os.ReadDir(rootPath)
		if errorValue != nil {
			if os.IsNotExist(errorValue) {
				continue
			}
			return nil, errorValue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillPath := filepath.Join(rootPath, entry.Name())
			if _, isDuplicate := pathByName[entry.Name()]; isDuplicate {
				return nil, fmt.Errorf("skill %q is provided by both %s and %s", entry.Name(), pathByName[entry.Name()], skillPath)
			}
			pathByName[entry.Name()] = skillPath
			skillDirectories = append(skillDirectories, SkillDirectory{Name: entry.Name(), Path: skillPath})
		}
	}
	sort.Slice(skillDirectories, func(first int, second int) bool {
		return skillDirectories[first].Name < skillDirectories[second].Name
	})
	return skillDirectories, nil
}

func ToolsPath(scriptDir string) string {
	return filepath.Join(AssetsPath(scriptDir), "tools")
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
