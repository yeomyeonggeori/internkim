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
		return mattermostBinaryPresent && mattermostURL != ""
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
