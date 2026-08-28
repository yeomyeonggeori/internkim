package blueclawworkspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
			if skillIsForExternalClients(skillPath) {
				continue
			}
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

const skillAudienceMetadataKey = "kim.intern.audience"
const skillAudienceClient = "client"

// A plugin may carry a skill addressed to external Agent Plugins clients
// rather than to the agent: the guest has no OS keychain to run it with, and
// the agent selecting a skill that drives its own public API is a loop. The
// skill says so under the frontmatter metadata map — the slot the Agent
// Skills schema keeps for harness keys — and every agent-workspace ship path
// enumerates through SkillDirectories, which leaves such a skill out.
func skillIsForExternalClients(skillDirectoryPath string) bool {
	document, errorValue := os.ReadFile(filepath.Join(skillDirectoryPath, "SKILL.md"))
	if errorValue != nil {
		return false
	}
	return skillFrontmatterMetadata(string(document))[skillAudienceMetadataKey] == skillAudienceClient
}

// Reads the flat key-value entries under `metadata:` in a SKILL.md
// frontmatter block. Only indented lines below the metadata key count, so a
// top-level key of the same name stays what the Agent Skills schema says it
// is: rejected.
func skillFrontmatterMetadata(document string) map[string]string {
	trimmedDocument := strings.TrimSpace(document)
	if !strings.HasPrefix(trimmedDocument, "---\n") {
		return nil
	}
	frontmatter, _, hasFrontmatter := strings.Cut(strings.TrimPrefix(trimmedDocument, "---\n"), "\n---")
	if !hasFrontmatter {
		return nil
	}
	values := map[string]string{}
	inMetadata := false
	for _, line := range strings.Split(frontmatter, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine == "" {
			continue
		}
		isIndented := line != strings.TrimLeft(line, " \t")
		if !isIndented {
			inMetadata = trimmedLine == "metadata:"
			continue
		}
		if !inMetadata {
			continue
		}
		key, value, hasKey := strings.Cut(trimmedLine, ":")
		if !hasKey {
			continue
		}
		values[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), `"'`)
	}
	return values
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
