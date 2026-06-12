package setup

import (
	"errors"
	"fmt"
	"path"

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
		return trimmedRun(context, blueclawPayloadManifestCheckCommand(localManifest)) == "ok"
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallBlueclawPayloadSSH == nil {
			return errors.New("blueclaw payload SSH callback missing")
		}
		if errorValue := context.Callbacks.InstallBlueclawPayloadSSH(context); errorValue != nil {
			return errorValue
		}
		if context.Backend == BackendSSH && context.SSH != nil {
			if serviceStatus := trimmedRun(context, "if systemctl cat "+blueclaw.BlueclawServiceName+" >/dev/null 2>&1; then systemctl restart "+blueclaw.BlueclawServiceName+" && systemctl is-active "+blueclaw.BlueclawServiceName+" 2>/dev/null; else echo not-installed; fi"); serviceStatus != "active" && serviceStatus != "not-installed" {
				return fmt.Errorf("blueclaw restart after payload deploy failed: %s", serviceStatus)
			}
		}
		return nil
	},
}

var StepBlueclawPayloadDirect = Step{
	Name:        "blueclaw-payload-direct",
	Title:       StepBlueclawPayload.Title,
	IsSatisfied: StepBlueclawPayload.IsSatisfied,
	Run:         StepBlueclawPayload.Run,
}

func blueclawPayloadManifestCheckCommand(localManifest string) string {
	return `set -eu
expected_manifest_path="$(mktemp /tmp/internkim-blueclaw-expected-manifest.XXXXXX)"
host_workspace_manifest_path="$(mktemp /tmp/internkim-blueclaw-host-workspace-manifest.XXXXXX)"
workspace_manifest_path="$(mktemp /tmp/internkim-blueclaw-workspace-manifest.XXXXXX)"
cleanup_blueclaw_payload_manifest_check() {
  rm -f "$expected_manifest_path" "$host_workspace_manifest_path" "$workspace_manifest_path"
}
trap cleanup_blueclaw_payload_manifest_check EXIT
printf '%s' ` + shellQuote(localManifest) + ` > "$expected_manifest_path"
cat ` + shellQuote(path.Join(blueclaw.BlueclawWorkspacePath, ".blueclaw", "runtime", "current", "manifest.json")) + ` > "$host_workspace_manifest_path" 2>/dev/null || true
debugfs -R ` + shellQuote("cat /.blueclaw/runtime/current/manifest.json") + ` ` + shellQuote(blueclaw.BlueclawWorkspaceImagePath) + ` > "$workspace_manifest_path" 2>/dev/null || true
if cmp -s "$expected_manifest_path" ` + shellQuote(blueclaw.BlueclawPayloadManifestPath) + ` && cmp -s "$expected_manifest_path" "$host_workspace_manifest_path" && cmp -s "$expected_manifest_path" "$workspace_manifest_path"; then
  echo ok
else
  echo missing
fi`
}
