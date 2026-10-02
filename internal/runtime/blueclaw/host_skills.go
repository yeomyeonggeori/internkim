package blueclaw

// SkillPreparationVerb is the `internkim` command that runs every bundled
// skill's setup, which keeps what the skill needs beside its own files. The
// install step runs it on every install and upgrade.
const SkillPreparationVerb = "prepare-skills"

// GuestSkillPreparationVerb is the same preparation for a device's guest. The
// device's delivery refresh runs it with internkim-admind, the one program of
// ours every device carries, inside the guest's root filesystem.
const GuestSkillPreparationVerb = "prepare-guest-skills"

// The guest's interpreter and the PATH its agent starts with: Debian's
// python3, and the login default that holds the uv and bun
// tools/prepare-blueclaw-runtime installs in /usr/local/bin.
const (
	blueclawGuestPythonPath = "/usr/bin/python3"
	blueclawGuestSearchPath = "/usr/local/bin:/usr/bin:/bin"
)

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

// GuestBundledSkillsPlace is the skills as the device's guest sees them,
// mounted read-only at /delivery/skills.
func GuestBundledSkillsPlace() BundledSkillsPlace {
	return BundledSkillsPlace{SkillsPath: BlueclawGuestDeliverySkillsPath, PythonPath: blueclawGuestPythonPath, SearchPath: blueclawGuestSearchPath}
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
