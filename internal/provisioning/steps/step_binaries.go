package setup

import (
	"errors"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBinaries = Step{
	Name: "binaries",
	Deps: []string{"board", "web"},
	Title: func(context *Context) string {
		return context.T("바이너리 설치 중...", "Installing binaries...")
	},
	IsSatisfied: func(context *Context) bool {
		version := ""
		if context.Callbacks.BinariesVersion != nil {
			version = context.Callbacks.BinariesVersion()
		}
		switch context.Backend {
		case BackendSSH:
			if !sshFileExists(context, "/usr/local/bin/blueclaw") ||
				!sshFileExists(context, "/usr/local/bin/blueclaw-llmd") ||
				!sshFileExists(context, "/usr/local/bin/blueclaw-supervisor") ||
				!sshFileExists(context, "/usr/local/bin/internkim-capabilityd") ||
				!sshFileExists(context, "/usr/local/bin/internkim-admind") ||
				!sshFileExists(context, "/usr/local/bin/internkim-local-llm-runner") ||
				!sshFileExists(context, "/usr/local/bin/pocketbase") ||
				!sshFileExists(context, "/usr/local/bin/graphiti-memoryd") ||
				!sshFileExists(context, blueclaw.BuzzRelayBinaryPath) ||
				!sshFileExists(context, blueclaw.BuzzAdminBinaryPath) {
				return false
			}
			if version == "" {
				return true
			}
			return trimmedRun(context, "cat /root/.internkim/state/binaries-version 2>/dev/null") == version
		case BackendSD:
			return stagedFileExists(context, "bin/blueclaw") &&
				stagedFileExists(context, "bin/blueclaw-llmd") &&
				stagedFileExists(context, "bin/internkim-capabilityd") &&
				stagedFileExists(context, "bin/internkim-admind") &&
				stagedFileExists(context, "bin/internkim-local-llm-runner")
		}
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallBinariesSSH == nil {
			return errors.New("binaries SSH callback missing")
		}
		return context.Callbacks.InstallBinariesSSH(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageBinariesSD == nil {
			return errors.New("binaries SD callback missing")
		}
		return context.Callbacks.StageBinariesSD(context)
	},
}
