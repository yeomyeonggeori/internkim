package blueclaw

// PreparedSkillsDirectoryName is the cache home the install step prepares the
// bundled skills in, beside them. Every person's command runs with a cache home
// of its own, so an environment prepared in any one account's cache would be
// found by nobody else; a directory that travels with the skills is found by
// every account that can read them.
const PreparedSkillsDirectoryName = ".prepared"

// SkillPreparationVerb is the `internkim` command that prepares every bundled
// skill. The install step runs it, and an administrator runs it again to add
// what a skill's setup leaves out unless asked, such as `--with-ocr`.
const SkillPreparationVerb = "prepare-skills"

func (layout CompanyHostLayout) CommandPath() string {
	return layout.CommandRoot + "/" + CompanyPackageName
}

func (layout CompanyHostLayout) PreparedSkillsPath() string {
	return layout.SkillsPath() + "/" + PreparedSkillsDirectoryName
}

func (layout CompanyHostLayout) SkillPreparationCommand() HostSetupCommand {
	return HostSetupCommand{
		Purpose:   "prepare every bundled skill in " + layout.SkillsPath(),
		Arguments: []string{layout.CommandPath(), SkillPreparationVerb},
	}
}

// InstallStepCommands is everything the package's install step runs, in order:
// the interpreter and the conversion environment first, because the skills are
// prepared on that interpreter.
func (layout CompanyHostLayout) InstallStepCommands() []HostSetupCommand {
	return append(layout.PythonSetupCommands(), layout.SkillPreparationCommand())
}
