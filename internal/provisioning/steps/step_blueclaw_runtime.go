package setup

import (
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBlueclawRuntime = Step{
	Name: "blueclaw-runtime",
	Deps: []string{"binaries", "skills"},
	Title: func(context *Context) string {
		return context.T("Blueclaw Firecracker 런타임 준비 중...", "Preparing Blueclaw Firecracker runtime...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return false
		}
		if context.Callbacks.BlueclawRuntimeManifest != nil {
			localManifest := context.Callbacks.BlueclawRuntimeManifest()
			if localManifest == "" {
				return false
			}
			if trimmedRun(context, "printf '%s' "+shellQuote(localManifest)+" | cmp -s - "+shellQuote(blueclaw.BlueclawRuntimeManifestPath)+" && echo ok || echo missing") != "ok" {
				return false
			}
		}
		return trimmedRun(context, "test -x "+shellQuote(blueclaw.BlueclawSupervisorBinaryPath)+" && "+
			"test -x "+shellQuote(blueclaw.BlueclawFirecrackerPath)+" && "+
			"test -x "+shellQuote(blueclaw.BlueclawJailerPath)+" && "+
			"test -s "+shellQuote(blueclaw.BlueclawKernelImagePath)+" && "+
			"test -s "+shellQuote(blueclaw.BlueclawRootFilesystemImagePath)+" && "+
			"test -s "+shellQuote(blueclaw.BlueclawRuntimeManifestPath)+" && "+
			"blkid -o value -s TYPE "+shellQuote(blueclaw.BlueclawWorkspaceImagePath)+" 2>/dev/null | grep -qx ext4 && echo ok || echo missing") == "ok" &&
			trimmedRun(context, blueclawRootfsBinaryContractCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallBlueclawRuntimeSSH == nil {
			return errors.New("blueclaw runtime SSH callback missing")
		}
		if errorValue := context.Callbacks.InstallBlueclawRuntimeSSH(context); errorValue != nil {
			return errorValue
		}
		if context.Backend == BackendSSH {
			if check := trimmedRun(context, blueclawRootfsBinaryContractCheckCommand()); check != "ok" {
				return fmt.Errorf("blueclaw rootfs binary contract drift: %s", check)
			}
		}
		return nil
	},
}
