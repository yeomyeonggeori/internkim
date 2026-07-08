package setup

import (
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const skillsManifestPath = "/root/.blueclaw/workspace/skills/.internkim-skills-manifest.json"

var StepSkills = Step{
	Name: "skills",
	Deps: []string{"blueclaw-runtime-base"},
	Title: func(context *Context) string {
		return context.T("Blueclaw 스킬 정리...", "Preparing Blueclaw skills...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			if context.Callbacks.SkillsManifest == nil {
				return false
			}
			localManifest := context.Callbacks.SkillsManifest()
			if localManifest == "" {
				return false
			}
			return sshFileExists(context, "/root/.blueclaw/workspace/skills/presentation/SKILL.md") &&
				trimmedRun(context, "printf '%s' "+shellQuote(localManifest)+" | cmp -s - "+shellQuote(skillsManifestPath)+" && echo ok || echo missing") == "ok"
		case BackendSD:
			return true
		}
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.InstallSkillsSSH == nil {
			return errors.New("skills SSH callback missing")
		}
		if errorValue := context.Callbacks.InstallSkillsSSH(context); errorValue != nil {
			return errorValue
		}
		if context.Backend == BackendSSH && context.SSH != nil {
			serviceStatus := trimmedRun(context, blueclawWorkspaceSkillsSyncCommand())
			if serviceStatus != "active" && serviceStatus != "missing" {
				return fmt.Errorf("blueclaw workspace skills sync failed: %s", serviceStatus)
			}
		}
		return nil
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageSkillsSD != nil {
			return context.Callbacks.StageSkillsSD(context)
		}
		fmt.Println("  " + context.T("firstboot이 첫 부팅에 처리합니다", "firstboot handles this on first boot"))
		return nil
	},
}

func blueclawWorkspaceSkillsSyncCommand() string {
	blueclawProcessPattern := `[/]usr/local/bin/blueclaw-supervisor|[/]firecracker .*--api-sock /firecracker-api.socket`
	return `set -eu
if ! systemctl cat ` + blueclaw.BlueclawServiceName + ` >/dev/null 2>&1; then
  echo missing
  exit 0
fi
systemctl stop ` + blueclaw.BlueclawServiceName + ` >/dev/null 2>&1 || true
for _ in $(seq 1 20); do
  if ! systemctl is-active --quiet ` + blueclaw.BlueclawServiceName + ` && ! pgrep -f ` + shellQuote(blueclawProcessPattern) + ` >/dev/null; then
    break
  fi
  sleep 1
done
if systemctl is-active --quiet ` + blueclaw.BlueclawServiceName + ` || pgrep -f ` + shellQuote(blueclawProcessPattern) + ` >/dev/null; then
  systemctl kill ` + blueclaw.BlueclawServiceName + ` --kill-who=all --signal=KILL >/dev/null 2>&1 || true
fi
` + blueclaw.BlueclawSupervisorBinaryPath + ` sync-workspace --workspace-image ` + shellQuote(blueclaw.BlueclawWorkspaceImagePath) + ` --source ` + shellQuote(blueclaw.BlueclawWorkspacePath) + `
systemctl start ` + blueclaw.BlueclawServiceName + `
systemctl is-active ` + blueclaw.BlueclawServiceName + ` 2>/dev/null`
}
