package setup

import (
	"errors"
	"fmt"
)

const skillsManifestPath = "/root/.blueclaw/workspace/skills/.internkim-skills-manifest.json"

var StepSkills = Step{
	Name: "skills",
	Deps: []string{"binaries"},
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
			return sshFileExists(context, "/root/.blueclaw/workspace/skills/simple-slides/SKILL.md") &&
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
		return context.Callbacks.InstallSkillsSSH(context)
	},
	RunSD: func(context *Context) error {
		if context.Callbacks.StageSkillsSD != nil {
			return context.Callbacks.StageSkillsSD(context)
		}
		fmt.Println("  " + context.T("firstboot이 첫 부팅에 처리합니다", "firstboot handles this on first boot"))
		return nil
	},
}
