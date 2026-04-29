package setup

import (
	"errors"
	"fmt"
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
		mattermostActive := trimmedRun(context, "systemctl is-active mattermost 2>/dev/null") == "active"
		mattermostResponding := trimmedRun(context, `curl -sf http://localhost:8065/api/v4/system/ping 2>/dev/null | grep -q '"status":"OK"' && echo ok || true`) == "ok"
		return mattermostBinaryPresent &&
			mattermostURL != "" &&
			(expectedMattermostURL == "" || mattermostURL == expectedMattermostURL) &&
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
