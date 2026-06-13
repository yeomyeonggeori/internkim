package setup

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBlueclawConfiguration = Step{
	Name: "blueclaw-config",
	Title: func(context *Context) string {
		return context.T("Blueclaw 런타임 설정 배포 중...", "Deploying Blueclaw runtime configuration...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return false
		}
		if !blueclawConfigurationFilesMatchGenerated(context) {
			return false
		}
		return trimmedRun(context, blueclawRuntimeContractCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.SSH == nil {
			return errors.New("blueclaw configuration SSH connection missing")
		}

		runtimeConfiguration, errorValue := generatedBlueclawRuntimeConfiguration(context)
		if errorValue != nil {
			return errorValue
		}
		policyConfiguration, errorValue := blueclaw.BlueclawPolicyDocument(loadGoogleEmail(context))
		if errorValue != nil {
			return errorValue
		}

		context.SSH.Run(buildBlueclawConfigurationDirectoryCommand())
		for _, file := range []struct {
			path    string
			content string
		}{
			{path: blueclaw.BlueclawRuntimeConfigPath, content: runtimeConfiguration},
			{path: blueclaw.BlueclawPolicyConfigPath, content: policyConfiguration},
			{path: blueclaw.BlueclawWorkspacePath + "/.blueclaw/config/runtime.json", content: runtimeConfiguration},
			{path: blueclaw.BlueclawWorkspacePath + "/.blueclaw/config/policy.json", content: policyConfiguration},
		} {
			if errorValue := uploadBlueclawConfigurationFile(context, file.path, file.content); errorValue != nil {
				return errorValue
			}
		}
		installOutput := context.SSH.Run(buildBlueclawConfigurationPermissionCommand() + "\necho blueclaw-config-installed")
		if !strings.Contains(installOutput, "blueclaw-config-installed") {
			return fmt.Errorf("blueclaw configuration install failed: %s", strings.TrimSpace(installOutput))
		}
		if trimmedRun(context, "systemctl cat "+blueclaw.BlueclawServiceName+" >/dev/null 2>&1 && echo present || echo missing") == "present" {
			if serviceStatus := trimmedRun(context, "systemctl restart "+blueclaw.BlueclawServiceName+" && systemctl is-active "+blueclaw.BlueclawServiceName+" 2>/dev/null"); serviceStatus != "active" {
				return fmt.Errorf("blueclaw restart after configuration deploy failed: %s", serviceStatus)
			}
		}

		if runtimeCheck := trimmedRun(context, blueclawRuntimeContractCheckCommand()); runtimeCheck != "ok" {
			return fmt.Errorf("blueclaw runtime configuration contract drift: %s", runtimeCheck)
		}

		return nil
	},
}

func blueclawConfigurationFilesMatchGenerated(context *Context) bool {
	runtimeConfiguration, errorValue := generatedBlueclawRuntimeConfiguration(context)
	if errorValue != nil {
		return false
	}
	policyConfiguration, errorValue := blueclaw.BlueclawPolicyDocument(loadGoogleEmail(context))
	if errorValue != nil {
		return false
	}
	return remoteFileMatchesContent(context, blueclaw.BlueclawRuntimeConfigPath, runtimeConfiguration) &&
		remoteFileMatchesContent(context, blueclaw.BlueclawPolicyConfigPath, policyConfiguration) &&
		remoteFileMatchesContent(context, blueclaw.BlueclawWorkspacePath+"/.blueclaw/config/runtime.json", runtimeConfiguration) &&
		remoteFileMatchesContent(context, blueclaw.BlueclawWorkspacePath+"/.blueclaw/config/policy.json", policyConfiguration)
}

func remoteFileMatchesContent(context *Context, path string, content string) bool {
	return trimmedRun(context, "printf '%s' "+shellQuote(content)+" | cmp -s - "+shellQuote(path)+" && echo ok || echo missing") == "ok"
}

func generatedBlueclawRuntimeConfiguration(context *Context) (string, error) {
	return blueclaw.BlueclawRuntimeConfigDocumentWithOptions(blueclaw.RuntimeConfigOptions{
		AdminTaskLinkBaseURL: loadDeviceURL(context),
	})
}

func loadDeviceURL(context *Context) string {
	if context.Callbacks.LoadState == nil {
		return ""
	}
	return context.Callbacks.LoadState("device_url")
}

func loadGoogleEmail(context *Context) string {
	if context.Callbacks.LoadState == nil {
		return ""
	}
	return context.Callbacks.LoadState("google_email")
}

func uploadBlueclawConfigurationFile(context *Context, path string, content string) error {
	file, errorValue := os.CreateTemp("", "internkim-blueclaw-config-*.json")
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	if _, errorValue := file.WriteString(content); errorValue != nil {
		file.Close()
		return errorValue
	}
	if errorValue := file.Close(); errorValue != nil {
		return errorValue
	}
	return context.SSH.SCP(temporaryPath, path)
}

func buildBlueclawConfigurationDirectoryCommand() string {
	return fmt.Sprintf(`mkdir -p %s %s`,
		blueclaw.BlueclawConfigPath,
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config",
	)
}

func buildBlueclawConfigurationPermissionCommand() string {
	return fmt.Sprintf(`set -e
chown -R root:blueclaw %s
chmod 770 %s
chmod 640 %s %s
chown -R blueclaw:blueclaw %s
chmod 750 %s
chmod 640 %s %s`,
		blueclaw.BlueclawConfigPath,
		blueclaw.BlueclawConfigPath,
		blueclaw.BlueclawRuntimeConfigPath,
		blueclaw.BlueclawPolicyConfigPath,
		blueclaw.BlueclawWorkspacePath+"/.blueclaw",
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config",
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config/runtime.json",
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config/policy.json",
	)
}
