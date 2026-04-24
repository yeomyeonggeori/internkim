package setup

import "errors"

var StepStaging = Step{
	Name: "staging",
	Deps: []string{"binaries", "openrouter", "tunnel", "google"},
	Title: func(context *Context) string {
		return context.T("부팅 스테이지 준비...", "Preparing boot staging payload...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSD:
			return stagedFileExists(context, "internkim-firstboot.sh") &&
				stagedFileExists(context, "config/runtime.json") &&
				stagedFileExists(context, "config/policy.json") &&
				stagedFileExists(context, "authorized_keys") &&
				stagedFileExists(context, "secrets/mm-admin-pass") &&
				stagedFileExists(context, "setup-build-id")
		case BackendSSH:
			return true
		}
		return false
	},
	Run: func(context *Context) error {
		return nil
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageBootstrapSD == nil {
			return errors.New("bootstrap SD callback missing")
		}
		return context.Callbacks.StageBootstrapSD(context)
	},
}
