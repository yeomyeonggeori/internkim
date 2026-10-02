package blueclaw

// SkillPreparationVerb is the `internkim` command that runs every bundled
// skill's setup, which keeps what the skill needs beside its own files. The
// install step runs it on every install and upgrade.
const SkillPreparationVerb = "prepare-skills"

// BundledSkillsPlace is where a machine keeps the skills it hands the agent,
// and the interpreter and PATH every person's command runs them with, which is
// what their setup has to prepare them on.
type BundledSkillsPlace struct {
	SkillsPath string
	PythonPath string
	SearchPath string
}

func (layout CompanyHostLayout) BundledSkillsPlace() BundledSkillsPlace {
	return BundledSkillsPlace{SkillsPath: layout.SkillsPath(), PythonPath: layout.PythonPath(), SearchPath: layout.SearchPath()}
}

func (layout CompanyHostLayout) CommandPath() string {
	return layout.CommandRoot + "/" + CompanyPackageName
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
