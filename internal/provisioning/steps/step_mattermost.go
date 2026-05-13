package setup

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

var StepMattermost = Step{
	Name: "mattermost",
	Deps: []string{"binaries", "tunnel"},
	Title: func(context *Context) string {
		return context.T("Mattermost 설치 및 설정...", "Installing and configuring Mattermost...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		mattermostBinaryPresent := sshFileExists(context, "/opt/mattermost/bin/mattermost")
		mattermostURL := trimmedRun(context, "cat /root/.internkim/env/mattermost-url 2>/dev/null")
		expectedMattermostURL := ""
		if context.Callbacks.LoadState != nil {
			expectedMattermostURL = context.Callbacks.LoadState("device_url")
		}
		mattermostManagedResourcesConfigured := trimmedRun(context, mattermostManagedResourceCheckCommand()) == "ok"
		mattermostActive := trimmedRun(context, "systemctl is-active mattermost 2>/dev/null") == "active"
		mattermostResponding := trimmedRun(context, `curl -sf http://localhost:8065/api/v4/system/ping 2>/dev/null | grep -q '"status":"OK"' && echo ok || true`) == "ok"
		return mattermostBinaryPresent &&
			mattermostURL != "" &&
			(expectedMattermostURL == "" || mattermostURL == expectedMattermostURL) &&
			mattermostManagedResourcesConfigured &&
			mattermostActive &&
			mattermostResponding
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallMattermost == nil || context.Callbacks.SetupMattermost == nil {
			return errors.New("mattermost callbacks missing")
		}
		if err := context.Callbacks.InstallMattermost(context); err != nil {
			return fmt.Errorf("install: %w", err)
		}
		if err := context.Callbacks.SetupMattermost(context); err != nil {
			return fmt.Errorf("setup: %w", err)
		}
		return nil
	},
}

func mattermostManagedResourceCheckCommand() string {
	return fmt.Sprintf(`python3 - <<'PY'
import json
from pathlib import Path

path = Path("/opt/mattermost/config/config.json")
document = json.loads(path.read_text())
configured_value = document.get("ServiceSettings", {}).get("ManagedResourcePaths", "")
configured_paths = {part.strip() for part in configured_value.split(",") if part.strip()}
required_paths = {%s}
service_settings = document.get("ServiceSettings", {})
tokens_enabled = service_settings.get("EnableUserAccessTokens") is True
bots_enabled = service_settings.get("EnableBotAccountCreation") is True
print("ok" if required_paths.issubset(configured_paths) and tokens_enabled and bots_enabled else "missing")
PY`, mattermostManagedResourcePythonSetValues())
}

func mattermostManagedResourcePythonSetValues() string {
	values := make([]string, 0, len(mattermostdefaults.ManagedResourcePaths()))
	for _, resourcePath := range mattermostdefaults.ManagedResourcePaths() {
		values = append(values, strconv.Quote(resourcePath))
	}
	return strings.Join(values, ", ")
}
