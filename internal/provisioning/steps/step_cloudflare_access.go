package setup

import "errors"

var StepCloudflareAccess = Step{
	Name:      "cloudflare-access",
	Deps:      []string{"web"},
	ForceDeps: []string{"web"},
	Title: func(context *Context) string {
		return context.T("Cloudflare Access 정책 동기화 중...", "Syncing Cloudflare Access policies...")
	},
	IsSatisfied: func(context *Context) bool {
		return true
	},
	Run: func(context *Context) error {
		if context.Callbacks.SyncCloudflareAccess == nil {
			return errors.New("cloudflare access sync callback missing")
		}
		return context.Callbacks.SyncCloudflareAccess(context)
	},
}
