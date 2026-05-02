package setup

import "fmt"

var StepSkills = Step{
	Name: "skills",
	Deps: []string{"binaries"},
	Title: func(context *Context) string {
		return context.T("Blueclaw 스킬 정리...", "Preparing Blueclaw skills...")
	},
	IsSatisfied: func(context *Context) bool {
		switch context.Backend {
		case BackendSSH:
			return sshFileExists(context, "/root/.blueclaw/workspace/skills/simple-slides/SKILL.md")
		case BackendSD:
			return true
		}
		return false
	},
	Run: func(context *Context) error {
		context.SSH.Run(`chown -R blueclaw:blueclaw /root/.blueclaw/workspace/skills 2>/dev/null || true`)
		return nil
	},
	RunSD: func(context *Context) error {
		fmt.Println("  " + context.T("firstboot이 첫 부팅에 처리합니다", "firstboot handles this on first boot"))
		return nil
	},
}
