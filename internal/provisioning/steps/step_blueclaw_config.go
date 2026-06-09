package setup

import (
	"errors"
	"fmt"

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

		runtimeConfiguration, errorValue := blueclaw.BlueclawRuntimeConfigDocument("")
		if errorValue != nil {
			return errorValue
		}
		policyConfiguration, errorValue := blueclaw.BlueclawPolicyDocument(loadGoogleEmail(context))
		if errorValue != nil {
			return errorValue
		}

		context.SSH.Run(buildBlueclawConfigurationInstallCommand(runtimeConfiguration, policyConfiguration))
		if serviceStatus := trimmedRun(context, "systemctl restart "+blueclaw.BlueclawServiceName+" && systemctl is-active "+blueclaw.BlueclawServiceName+" 2>/dev/null"); serviceStatus != "active" {
			return fmt.Errorf("blueclaw restart after configuration deploy failed: %s", serviceStatus)
		}

		if runtimeCheck := trimmedRun(context, blueclawRuntimeContractCheckCommand()); runtimeCheck != "ok" {
			return fmt.Errorf("blueclaw runtime configuration contract drift: %s", runtimeCheck)
		}

		return nil
	},
}

func blueclawConfigurationFilesMatchGenerated(context *Context) bool {
	runtimeConfiguration, errorValue := blueclaw.BlueclawRuntimeConfigDocument("")
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

func loadGoogleEmail(context *Context) string {
	if context.Callbacks.LoadState == nil {
		return ""
	}
	return context.Callbacks.LoadState("google_email")
}

func buildBlueclawConfigurationInstallCommand(runtimeConfiguration string, policyConfiguration string) string {
	return fmt.Sprintf(`mkdir -p %s %s
printf '%%s' %s > %s
printf '%%s' %s > %s
mkdir -p %s
printf '%%s' %s > %s
printf '%%s' %s > %s
chown -R root:blueclaw %s
chmod 770 %s
chmod 640 %s %s
chown -R blueclaw:blueclaw %s
chmod 750 %s
chmod 640 %s %s`,
		blueclaw.BlueclawConfigPath,
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config",
		shellQuote(runtimeConfiguration),
		blueclaw.BlueclawRuntimeConfigPath,
		shellQuote(policyConfiguration),
		blueclaw.BlueclawPolicyConfigPath,
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config",
		shellQuote(runtimeConfiguration),
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config/runtime.json",
		shellQuote(policyConfiguration),
		blueclaw.BlueclawWorkspacePath+"/.blueclaw/config/policy.json",
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
