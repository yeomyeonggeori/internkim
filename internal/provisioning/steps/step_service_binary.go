package setup

import "errors"

var StepAdmind = Step{
	Name: "admind",
	Deps: []string{"board"},
	Title: func(context *Context) string {
		return context.T("admind 배포 중...", "Deploying admind...")
	},
	IsSatisfied: func(context *Context) bool {
		return sshFileExists(context, "/usr/local/bin/internkim-admind") &&
			trimmedRun(context, "systemctl is-active internkim-admind 2>/dev/null") == "active"
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallAdmindSSH == nil {
			return errors.New("admind SSH callback missing")
		}
		return context.Callbacks.InstallAdmindSSH(context)
	},
}

var StepCapabilityd = Step{
	Name: "capabilityd",
	Deps: []string{"board"},
	Title: func(context *Context) string {
		return context.T("capabilityd 배포 중...", "Deploying capabilityd...")
	},
	IsSatisfied: func(context *Context) bool {
		return sshFileExists(context, "/usr/local/bin/internkim-capabilityd") &&
			trimmedRun(context, "systemctl is-active internkim-capabilityd 2>/dev/null") == "active"
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallCapabilitydSSH == nil {
			return errors.New("capabilityd SSH callback missing")
		}
		return context.Callbacks.InstallCapabilitydSSH(context)
	},
}
