package setup

import (
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBlueclawRuntimeBase = Step{
	Name: "blueclaw-runtime-base",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("Blueclaw Firecracker 베이스 런타임 준비 중...", "Preparing Blueclaw Firecracker base runtime...")
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
			trimmedRun(context, blueclawRootfsBaseContractCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallBlueclawRuntimeSSH == nil {
			return errors.New("blueclaw runtime base SSH callback missing")
		}
		if errorValue := context.Callbacks.InstallBlueclawRuntimeSSH(context); errorValue != nil {
			return errorValue
		}
		if context.Backend == BackendSSH {
			if check := trimmedRun(context, blueclawRootfsBaseContractCheckCommand()); check != "ok" {
				return fmt.Errorf("blueclaw rootfs base contract drift: %s", check)
			}
		}
		return nil
	},
}

var StepBlueclawPayload = Step{
	Name: "blueclaw-payload",
	Deps: []string{"binaries", "skills", "blueclaw-runtime-base", "blueclaw-config"},
	Title: func(context *Context) string {
		return context.T("Blueclaw 런타임 페이로드 배포 중...", "Deploying Blueclaw runtime payload...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return false
		}
		if context.Callbacks.BlueclawPayloadManifest == nil {
			return false
		}
		localManifest := context.Callbacks.BlueclawPayloadManifest()
		if localManifest == "" {
			return false
		}
		return trimmedRun(context, "printf '%s' "+shellQuote(localManifest)+" | cmp -s - "+shellQuote(blueclaw.BlueclawPayloadManifestPath)+" && echo ok || echo missing") == "ok"
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallBlueclawPayloadSSH == nil {
			return errors.New("blueclaw payload SSH callback missing")
		}
		if errorValue := context.Callbacks.InstallBlueclawPayloadSSH(context); errorValue != nil {
			return errorValue
		}
		if context.Backend == BackendSSH && context.SSH != nil {
			if serviceStatus := trimmedRun(context, "systemctl restart "+blueclaw.BlueclawServiceName+" && systemctl is-active "+blueclaw.BlueclawServiceName+" 2>/dev/null"); serviceStatus != "active" {
				return fmt.Errorf("blueclaw restart after payload deploy failed: %s", serviceStatus)
			}
		}
		return nil
	},
}
