package setup

import "errors"

var StepBinaries = Step{
	Name: "binaries",
	Deps: []string{"board"},
	Title: func(context *Context) string {
		return context.T("바이너리 설치 중...", "Installing binaries...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return sshFileExists(context, "/usr/local/bin/blueclaw") &&
				sshFileExists(context, "/usr/local/bin/gws")
		case BackendSD:
			return stagedFileExists(context, "bin/blueclaw")
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
