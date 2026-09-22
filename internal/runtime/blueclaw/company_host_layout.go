package blueclaw

import "strings"

// Where the company host's own files sit is the only thing that differs between
// the two machines that run it. Debian's answer is the FHS one dpkg is allowed to
// write; a Mac's is Homebrew's prefix, because a formula installs into a cellar
// and is symlinked from there, and because `/` on macOS 11 and newer is a sealed
// read-only volume — `mkdir /workspace` fails with "Read-only file system", so
// the one absolute path the bundle invented for itself cannot exist there.
//
// Everything else is the same on both: /var/lib/internkim and /etc/internkim are
// on the writable data volume through firmlinks, /run has no macOS counterpart
// and CompanyHostRunPath moves with the rest of the state, and the addresses,
// ports and unit names are the constants above.
type CompanyHostLayout struct {
	// Name is what a refusal calls this machine.
	Name string
	// BinaryRoot is where a program the package ships is found.
	BinaryRoot string
	// HelperRoot holds the setuid helper and the preparation script, which
	// nobody types and which therefore do not belong beside the commands.
	HelperRoot string
	// LibraryRoot holds the skills, the migrations, the runtime template and
	// the document interpreter.
	LibraryRoot string
	// WorkspacePath is the root of the tree POSIX permissions are the boundary
	// of. It is never inside CompanyHostStateRoot, which is 0700 root:root and
	// which no task user can traverse.
	WorkspacePath string
	// RunPath holds what the preparation service stages for one boot.
	RunPath string
	// AgentHomePath is the home directory of the unprivileged account the agent
	// runs as. /home is an autofs mount point on macOS, so it is not one there.
	AgentHomePath string
}

// The macOS workspace is a sibling of the state root rather than a child, for
// the reason CompanyHostRelayStateDirectoryName already gives: the state root is
// 0700 root:root and an unprivileged account cannot open its own directory
// inside it.
const (
	macCompanyHostWorkspacePath = "/var/lib/internkim-workspace"
	macCompanyHostRunPath       = "/var/run/internkim"
	macBlueclawHomePath         = "/var/lib/blueclaw"

	companyHostPrepareProgramName = "prepare-company-host"
)

// DebianCompanyHostLayout is what the .deb installs, and what every systemd unit
// in this package names.
func DebianCompanyHostLayout() CompanyHostLayout {
	return CompanyHostLayout{
		Name:          "Debian",
		BinaryRoot:    CompanyPackageBinaryRoot,
		HelperRoot:    CompanyPackageHelperRoot,
		LibraryRoot:   CompanyPackageLibraryRoot,
		WorkspacePath: CompanyHostWorkspacePath,
		RunPath:       CompanyHostRunPath,
		AgentHomePath: BlueclawHomePath,
	}
}

// MacCompanyHostLayout is what `brew install internkim` leaves behind. The prefix
// is Homebrew's own answer rather than a literal, because it is /opt/homebrew on
// Apple silicon, /usr/local on Intel, and anything at all in a prefix somebody
// chose.
func MacCompanyHostLayout(homebrewPrefix string) CompanyHostLayout {
	prefix := strings.TrimRight(homebrewPrefix, "/")
	return CompanyHostLayout{
		Name:          "macOS",
		BinaryRoot:    prefix + "/bin",
		HelperRoot:    prefix + "/libexec/internkim",
		LibraryRoot:   prefix + "/share/internkim",
		WorkspacePath: macCompanyHostWorkspacePath,
		RunPath:       macCompanyHostRunPath,
		AgentHomePath: macBlueclawHomePath,
	}
}

// BinaryPath is where the package puts a program it ships.
func (layout CompanyHostLayout) BinaryPath(programName string) string {
	return layout.BinaryRoot + "/" + programName
}

// POSIXHelperPath is the one setuid file, which nobody runs from a shell: the
// rendered runtime document names it and blueclaw execs it.
func (layout CompanyHostLayout) POSIXHelperPath() string {
	return layout.HelperRoot + "/" + POSIXHelperProgramName
}

// PrepareScriptPath is the half of host/entrypoint.sh the supervisor does not
// take over.
func (layout CompanyHostLayout) PrepareScriptPath() string {
	return layout.HelperRoot + "/" + companyHostPrepareProgramName
}

func (layout CompanyHostLayout) SkillsPath() string {
	return layout.LibraryRoot + "/skills"
}

func (layout CompanyHostLayout) MigrationPath() string {
	return layout.LibraryRoot + "/migrations"
}

func (layout CompanyHostLayout) RuntimeTemplatePath() string {
	return layout.LibraryRoot + "/runtime.template.json"
}

func (layout CompanyHostLayout) DocumentVirtualEnvironmentPath() string {
	return layout.LibraryRoot + "/document-venv"
}

func (layout CompanyHostLayout) DocumentPythonPath() string {
	return layout.DocumentVirtualEnvironmentPath() + "/bin/python"
}

func (layout CompanyHostLayout) RuntimeDocumentPath() string {
	return layout.RunPath + "/runtime.json"
}

func (layout CompanyHostLayout) PolicyDocumentPath() string {
	return layout.RunPath + "/policy.json"
}

func (layout CompanyHostLayout) ACPSocketPath() string {
	return layout.RunPath + "/blueclaw-acp.sock"
}

func (layout CompanyHostLayout) RunSecretsPath() string {
	return layout.RunPath + "/secrets"
}

func (layout CompanyHostLayout) RunAgentKeyPath() string {
	return layout.RunSecretsPath() + "/agent-key"
}

func (layout CompanyHostLayout) RunModelKeyPath() string {
	return layout.RunSecretsPath() + "/openrouter-key"
}

func (layout CompanyHostLayout) CapabilitySocketPath() string {
	return layout.RunPath + "/capability.sock"
}
