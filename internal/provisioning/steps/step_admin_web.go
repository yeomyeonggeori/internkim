package setup

import "errors"

var StepAdminWeb = Step{
	Name: "admin-web",
	Deps: []string{"board"},
	Title: func(context *Context) string {
		return context.T("관리자 웹 배포 중...", "Deploying admin web...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Callbacks.AdminWebVersion == nil || context.Callbacks.LoadState == nil {
			return false
		}
		version := context.Callbacks.AdminWebVersion()
		return version != "" && context.Callbacks.LoadState("admin_web_version") == version
	},
	Run: func(context *Context) error {
		if context.Callbacks.DeployAdminWeb == nil {
			return errors.New("admin web deploy callback missing")
		}
		return context.Callbacks.DeployAdminWeb(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.DeployAdminWeb == nil {
			return errors.New("admin web deploy callback missing")
		}
		return context.Callbacks.DeployAdminWeb(context)
	},
}
