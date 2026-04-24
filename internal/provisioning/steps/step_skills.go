package setup

import (
	"fmt"
	"strings"
)

var StepSkills = Step{
	Name: "skills",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("gws-cli 스킬 심링크...", "Symlinking gws-cli skills...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return trimmedRun(context, "ls -d /root/.blueclaw/workspace/skills/gws-* 2>/dev/null | wc -l") != "0"
		case BackendSD:
			return true
		}
		return false
	},
	Run: func(context *Context) error {
		if context.Callbacks.GwsSkillsInstallScript == "" {
			return nil
		}
		if output := strings.TrimSpace(context.SSH.Run(context.Callbacks.GwsSkillsInstallScript)); output != "" {
			fmt.Println("  " + output)
		}
		return nil
	},
	RunSD: func(context *Context) error {
		fmt.Println("  " + context.T("firstboot이 첫 부팅에 처리합니다", "firstboot handles this on first boot"))
		return nil
	},
}
