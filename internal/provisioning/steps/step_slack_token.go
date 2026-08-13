package setup

import "errors"

var StepSlackToken = Step{
	Name: "slack",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("Slack 토큰 설정...", "Configuring Slack token...")
	},
	IsSatisfied: func(context *Context) bool {
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.ConfigureSlackTokenSSH == nil {
			return errors.New("slack token SSH callback missing")
		}
		return context.Callbacks.ConfigureSlackTokenSSH(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageSlackTokenSD == nil {
			return errors.New("slack token SD callback missing")
		}
		return context.Callbacks.StageSlackTokenSD(context)
	},
}
