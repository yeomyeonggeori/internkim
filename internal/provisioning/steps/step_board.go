package setup

import "fmt"

var StepBoard = Step{
	Name: "board",
	Deps: []string{"preflight"},
	Title: func(context *Context) string {
		return context.T("대상 확인", "Target verified")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return context.SSH != nil && context.BoardIP != ""
		case BackendSD:
			return context.SD != nil && context.SD.RootPath() != ""
		}
		return false
	},
	Run: func(context *Context) error {
		fmt.Printf("  %s: %s (%s)\n", context.T("타겟", "Target"), context.BoardIP, context.Backend)
		return nil
	},
	RunSD: func(context *Context) error {
		fmt.Printf("  %s: %s (%s)\n", context.T("타겟", "Target"), context.SD.RootPath(), context.Backend)
		return nil
	},
}
