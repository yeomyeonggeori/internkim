package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func runModel() {
	sub := ""
	if len(os.Args) > 2 {
		sub = os.Args[2]
	}

	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	target := resolveCommandTarget(commandControlArguments(os.Args[2:]))
	target = resolveLabHostForCommandTarget(target, scriptDir)
	ssh, _, errorValue := resolveDeviceSSHConnection(configuration, target)
	if errorValue != nil {
		fatal(errorValue.Error())
	}

	switch sub {
	case "current", "":
		modelCurrentCmd(ssh)
	case "set":
		if len(os.Args) < 4 {
			fmt.Println("Usage: internkim model set <model-id>")
			fmt.Println("Example: internkim model set " + blueclaw.BlueclawDefaultModelName)
			os.Exit(1)
		}
		modelSetCmd(ssh, os.Args[3])
	case "list":
		modelListCmd(ssh)
	default:
		fmt.Println("Usage: internkim model <current|set|list>")
	}
}

func runSyncTools() {
	configuration := loadConfig()
	scriptDir, _ := os.Getwd()
	target := resolveCommandTarget(commandControlArguments(os.Args[2:]))
	target = resolveLabHostForCommandTarget(target, scriptDir)
	ssh, _, errorValue := resolveDeviceSSHConnection(configuration, target)
	if errorValue != nil {
		fatal(errorValue.Error())
	}
	syncToolDescriptorsCmd(ssh)
}

func syncToolDescriptorsCmd(ssh *sshClient) {
	currentDocument := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	if err := json.Unmarshal([]byte(currentDocument), &document); err != nil {
		fatal("Failed to parse blueclaw runtime config: " + err.Error())
	}
	capabilitiesSection, _ := document["capabilities"].(map[string]any)
	if capabilitiesSection == nil {
		fatal("blueclaw runtime config has no capabilities section")
	}
	delete(capabilitiesSection, "toolNames")
	capabilitiesSection["toolDescriptors"] = capabilities.DefaultToolDescriptors()
	updatedDocument, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fatal("Failed to write blueclaw runtime config: " + err.Error())
	}
	temporaryPath := filepath.Join(os.TempDir(), "blueclaw-runtime.json")
	if err := os.WriteFile(temporaryPath, append(updatedDocument, '\n'), 0o600); err != nil {
		fatal("Failed to stage blueclaw runtime config: " + err.Error())
	}
	defer os.Remove(temporaryPath)
	for _, runtimeConfigPath := range blueclawRuntimeConfigPaths() {
		if errorValue := ssh.scp(temporaryPath, runtimeConfigPath); errorValue != nil {
			fatal("Failed to upload blueclaw runtime config: " + errorValue.Error())
		}
	}
	ssh.run("chown root:" + blueclaw.BlueclawUser + " " + quoteShellValues(blueclawRuntimeConfigPaths()) + " && chmod 640 " + quoteShellValues(blueclawRuntimeConfigPaths()))
	ssh.run("systemctl restart " + blueclaw.BlueclawServiceName + " 2>/dev/null")
	fmt.Printf("Synced %d tool descriptors to blueclaw runtime config.\n", len(capabilities.DefaultToolDescriptors()))
	fmt.Println("blueclaw restarted.")
}

func modelCurrentCmd(ssh *sshClient) {
	raw := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	_ = json.Unmarshal([]byte(raw), &document)
	model := blueclawRuntimeModel(document)
	if model == "" {
		fmt.Println("No model configured.")
		return
	}
	fmt.Printf("Model: %s\n", model)
	workspaceModel := strings.TrimSpace(ssh.run("jq -r '.languageModel.capability.model // empty' " + blueclawWorkspaceRuntimeConfigPath() + " 2>/dev/null"))
	if workspaceModel != "" && workspaceModel != model {
		fmt.Printf("Workspace model differs: %s\n", workspaceModel)
	}
}

func modelSetCmd(ssh *sshClient, modelID string) {
	currentDocument := strings.TrimSpace(ssh.run("cat " + blueclaw.BlueclawRuntimeConfigPath + " 2>/dev/null"))
	var document map[string]any
	if err := json.Unmarshal([]byte(currentDocument), &document); err != nil {
		fatal("Failed to parse blueclaw runtime config: " + err.Error())
	}
	languageModel, _ := document["languageModel"].(map[string]any)
	if languageModel == nil {
		languageModel = map[string]any{}
		document["languageModel"] = languageModel
	}
	capabilityModel, _ := languageModel["capability"].(map[string]any)
	if capabilityModel == nil {
		capabilityModel = map[string]any{}
		languageModel["capability"] = capabilityModel
	}
	capabilityModel["model"] = modelID
	updatedDocument, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		fatal("Failed to write blueclaw runtime config: " + err.Error())
	}
	temporaryPath := filepath.Join(os.TempDir(), "blueclaw-runtime.json")
	if err := os.WriteFile(temporaryPath, append(updatedDocument, '\n'), 0o600); err != nil {
		fatal("Failed to stage blueclaw runtime config: " + err.Error())
	}
	defer os.Remove(temporaryPath)
	for _, runtimeConfigPath := range blueclawRuntimeConfigPaths() {
		if errorValue := ssh.scp(temporaryPath, runtimeConfigPath); errorValue != nil {
			fatal("Failed to upload blueclaw runtime config: " + errorValue.Error())
		}
	}
	ssh.run("chown root:" + blueclaw.BlueclawUser + " " + quoteShellValues(blueclawRuntimeConfigPaths()) + " && chmod 640 " + quoteShellValues(blueclawRuntimeConfigPaths()))
	ssh.run("systemctl restart " + blueclaw.BlueclawServiceName + " 2>/dev/null")
	fmt.Printf("Model changed to: %s\n", modelID)
	fmt.Println("blueclaw restarted.")
}

func blueclawRuntimeModel(document map[string]any) string {
	if languageModel, ok := document["languageModel"].(map[string]any); ok {
		if capabilityModel, ok := languageModel["capability"].(map[string]any); ok {
			if value, ok := capabilityModel["model"].(string); ok {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func blueclawRuntimeConfigPaths() []string {
	return []string{
		blueclaw.BlueclawRuntimeConfigPath,
		blueclawWorkspaceRuntimeConfigPath(),
	}
}

func blueclawWorkspaceRuntimeConfigPath() string {
	return blueclaw.BlueclawWorkspacePath + "/.blueclaw/config/runtime.json"
}

func quoteShellValues(values []string) string {
	quotedValues := make([]string, 0, len(values))
	for _, value := range values {
		quotedValues = append(quotedValues, quoteShellValue(value))
	}
	return strings.Join(quotedValues, " ")
}

func modelListCmd(ssh *sshClient) {
	apiKey := strings.TrimPrefix(strings.TrimSpace(ssh.run("cat /root/.internkim/secrets/openrouter-api-key 2>/dev/null")), "OPENROUTER_API_KEY=")
	if apiKey == "" {
		fatal("No OpenRouter API key found on board.")
	}

	req, _ := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		fatal("Failed to fetch models: " + err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var result struct {
		Data []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fatal("Failed to parse models: " + err.Error())
	}

	// Show popular free/cheap models
	keywords := []string{"gemini", "flash", "qwen", "llama", "mistral", "deepseek", "gemma"}
	fmt.Printf("%-50s %s\n", "MODEL ID", "NAME")
	fmt.Println(strings.Repeat("-", 80))
	count := 0
	for _, m := range result.Data {
		id := strings.ToLower(m.ID)
		for _, kw := range keywords {
			if strings.Contains(id, kw) {
				fmt.Printf("%-50s %s\n", m.ID, m.Name)
				count++
				break
			}
		}
		if count >= 30 {
			break
		}
	}
	fmt.Printf("\n%d models shown. Use 'internkim model set <model-id>' to switch.\n", count)
}
