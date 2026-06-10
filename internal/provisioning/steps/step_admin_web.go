package setup

import "errors"

var StepAdminWeb = Step{
	Name: "web",
	Title: func(context *Context) string {
		return context.T("웹 배포 중...", "Deploying web...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Callbacks.AdminWebVersion == nil || context.Callbacks.LoadState == nil {
			return false
		}
		version := context.Callbacks.AdminWebVersion()
		return version != "" && (context.Callbacks.LoadState("web_version") == version || context.Callbacks.LoadState("admin_web_version") == version)
	},
	Run: func(context *Context) error {
		if context.Callbacks.DeployAdminWeb == nil {
			return errors.New("web deploy callback missing")
		}
		return context.Callbacks.DeployAdminWeb(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.DeployAdminWeb == nil {
			return errors.New("web deploy callback missing")
		}
		return context.Callbacks.DeployAdminWeb(context)
	},
}
