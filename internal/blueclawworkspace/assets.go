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

func dependencyPath(scriptDir string) string {
	return filepath.Join(scriptDir, ".dependency")
}

func PluginPaths(scriptDir string) []string {
	entries, errorValue := os.ReadDir(dependencyPath(scriptDir))
	if errorValue != nil {
		return nil
	}
	pluginPaths := []string{}
	for _, entry := range entries {
		pluginPath := filepath.Join(dependencyPath(scriptDir), entry.Name())
		if isExistingFile(filepath.Join(pluginPath, "plugin.json")) {
			pluginPaths = append(pluginPaths, pluginPath)
		}
	}
	sort.Strings(pluginPaths)
	return pluginPaths
}

func PluginSkillPaths(scriptDir string) []string {
	skillPaths := []string{}
	for _, pluginPath := range PluginPaths(scriptDir) {
		skillPaths = append(skillPaths, filepath.Join(pluginPath, "skills"))
	}
	return skillPaths
}

func SkillRootPaths(scriptDir string) []string {
	return PluginSkillPaths(scriptDir)
}

func isExistingFile(path string) bool {
	information, errorValue := os.Stat(path)
	return errorValue == nil && information.Mode().IsRegular()
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
