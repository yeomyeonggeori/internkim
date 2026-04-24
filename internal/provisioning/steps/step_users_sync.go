package setup

import "errors"

var StepUsersSync = Step{
	Name: "users-sync",
	Deps: []string{"services"},
	Title: func(context *Context) string {
		return context.T("초대 목록 동기화...", "Syncing invited users...")
	},
	IsSatisfied: func(context *Context) bool {
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallUsersSyncSSH == nil {
			return errors.New("users sync SSH callback missing")
		}
		return context.Callbacks.InstallUsersSyncSSH(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageUsersSyncSD == nil {
			return errors.New("users sync SD callback missing")
		}
		return context.Callbacks.StageUsersSyncSD(context)
	},
}
