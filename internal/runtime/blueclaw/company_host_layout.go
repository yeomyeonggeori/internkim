package blueclaw

import "strings"

// Where the company host's own files sit is the only thing that differs between
// the two machines that run it. Linux's answer is the FHS one a package manager is
// allowed to write; a Mac's is Homebrew's prefix, because a formula installs into a cellar
// and is symlinked from there, and because `/` on macOS 11 and newer is a sealed
// read-only volume — `mkdir /workspace` fails with "Read-only file system", so
// the one absolute path the bundle invented for itself cannot exist there.
//
// Everything else is the same on both: /var/lib/internkim and /etc/internkim are
// on the writable data volume through firmlinks, /run has no macOS counterpart
// and CompanyHostRunPath moves with the rest of the state, and the addresses,
// ports and unit names are the constants above.
type CompanyHostLayout struct {
	// BinaryRoot is where a program the package ships is found.
	BinaryRoot string
	// CommandRoot holds the one command a person types, `internkim`.
	CommandRoot string
	// HelperRoot holds the setuid helper and the preparation script, which
	// nobody types and which therefore do not belong beside the commands.
	HelperRoot string
	// LibraryRoot holds the skills, the migrations, the runtime template and
	// the document interpreter.
	LibraryRoot string
	// MigrationPath is where the package puts the database migrations the
	// runtime document names.
	MigrationPath string
	// WorkspacePath is the root of the tree POSIX permissions are the boundary
	// of. It is never inside CompanyHostStateRoot, which is 0700 root:root and
	// which no task user can traverse.
	WorkspacePath string
	// RunPath holds what the preparation service stages for one boot.
	RunPath string
	// AgentHomePath is the home directory of the unprivileged account the agent
	// runs as. /home is an autofs mount point on macOS, so it is not one there.
	AgentHomePath string
	// ProgramDirectories are where the services find the programs they shell
	// out to, after the host's own Python. launchd's default is
	// /usr/bin:/bin:/usr/sbin:/sbin, which holds neither Homebrew's prefix nor
	// this package's own tree, so a Mac names both.
	ProgramDirectories []string
	// DatabaseSocketDirectory and CacheSocketPath are where the host's own
	// database and cache answer, on a machine whose supervisor the package
	// installs units into. Empty means the machine's own PostgreSQL and Redis
	// are used, at DatabaseLoopbackAddress and the cache's loopback address.
	DatabaseSocketDirectory string
	CacheSocketPath         string
	DatabaseLoopbackAddress string
}

// The macOS workspace is a sibling of the state root rather than a child, for
// the reason CompanyHostRelayStateDirectoryName already gives: the state root is
// 0700 root:root and an unprivileged account cannot open its own directory
// inside it.
const (
	macCompanyHostWorkspacePath = "/var/lib/internkim-workspace"
	macCompanyHostRunPath       = "/var/run/internkim"
	macBlueclawHomePath         = "/var/lib/blueclaw"
	macDatabaseLoopbackAddress  = "127.0.0.1:5432"

	companyHostPrepareProgramName = "prepare-company-host"
)

// LinuxCompanyHostLayout is what every Linux package installs, and what every
// systemd unit in this package names.
func LinuxCompanyHostLayout() CompanyHostLayout {
	return CompanyHostLayout{
		BinaryRoot:    CompanyPackageBinaryRoot,
		CommandRoot:   CompanyPackageBinaryRoot,
		HelperRoot:    CompanyPackageHelperRoot,
		LibraryRoot:   CompanyPackageLibraryRoot,
		MigrationPath: CompanyPackageMigrationPath,
		WorkspacePath: CompanyHostWorkspacePath,
		RunPath:       CompanyHostRunPath,
		AgentHomePath: BlueclawHomePath,

		ProgramDirectories: []string{"/usr/sbin", "/usr/bin", "/sbin", "/bin"},

		DatabaseSocketDirectory: CompanyHostDatabaseSocketDirectory,
		CacheSocketPath:         CompanyHostCacheSocketPath,
	}
}

// MacCompanyHostLayout is what `brew install internkim` leaves behind. The prefix
// is Homebrew's own answer rather than a literal, because it is /opt/homebrew on
// Apple silicon, /usr/local on Intel, and anything at all in a prefix somebody
// chose.
//
// Everything sits under <prefix>/opt/internkim, the link Homebrew repoints at
// the current keg, so an upgrade that replaces the keg does not move a path a
// plist names. It is libexec rather than bin for a reason `brew link` gives out
// loud: the package vendors bun, uv and agent-browser, Homebrew has a formula
// for each of those names, and a keg that put them in bin could not be linked at
// all — "Target /opt/homebrew/bin/agent-browser is a symlink belonging to
// agent-browser". Only `internkim` is a command a person types, and it is the
// only thing the keg puts in bin. Homebrew does not link libexec into the
// prefix, which is how a formula ships a private tree without scattering
// "skills" and "migrations" into a directory every other formula shares.
//
// The setuid helper is in that tree rather than copied out of it, so a `brew
// upgrade` that replaces the keg drops the bit and the agent fails loudly until
// `sudo internkim install` is run again, instead of a new agent quietly running
// an old helper.
func MacCompanyHostLayout(homebrewPrefix string) CompanyHostLayout {
	prefix := strings.TrimRight(homebrewPrefix, "/")
	keg := prefix + "/opt/" + CompanyPackageName
	return CompanyHostLayout{
		BinaryRoot:    keg + "/libexec",
		CommandRoot:   keg + "/bin",
		HelperRoot:    keg + "/libexec",
		LibraryRoot:   keg + "/libexec",
		MigrationPath: keg + "/libexec/migrations",
		ProgramDirectories: []string{
			keg + "/libexec", prefix + "/bin", prefix + "/sbin",
			"/usr/bin", "/bin", "/usr/sbin", "/sbin",
		},
		WorkspacePath: macCompanyHostWorkspacePath,
		RunPath:       macCompanyHostRunPath,
		AgentHomePath: macBlueclawHomePath,

		DatabaseLoopbackAddress: macDatabaseLoopbackAddress,
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

// PrepareScriptPath is the script that prepares the runtime document, the
// roster and the staged keys before the supervisor starts the services.
func (layout CompanyHostLayout) PrepareScriptPath() string {
	return layout.HelperRoot + "/" + companyHostPrepareProgramName
}

func (layout CompanyHostLayout) SkillsPath() string {
	return layout.LibraryRoot + "/skills"
}

func (layout CompanyHostLayout) RuntimeTemplatePath() string {
	return layout.LibraryRoot + "/runtime.template.json"
}

func (layout CompanyHostLayout) AgentProfilePicturePath() string {
	return layout.LibraryRoot + "/agent-profile-picture.png"
}

// SearchPath is the PATH every service is started with. The agent hands it on
// to every command a requester runs, so the host's own Python comes first and
// a requester's `python3` is that interpreter on every machine the package
// installs on.
func (layout CompanyHostLayout) SearchPath() string {
	return strings.Join(append([]string{layout.PythonCommandsPath()}, layout.ProgramDirectories...), ":")
}

func (layout CompanyHostLayout) PythonRoot() string {
	return layout.LibraryRoot + "/python"
}

// PythonCommandsPath holds python3 and python, which lead to the interpreter
// through uv's minor-version link, so the path does not name a build and a
// patch release does not move it.
func (layout CompanyHostLayout) PythonCommandsPath() string {
	return layout.PythonRoot() + "/bin"
}

func (layout CompanyHostLayout) PythonPath() string {
	return layout.PythonCommandsPath() + "/python3"
}

func (layout CompanyHostLayout) DocumentVirtualEnvironmentPath() string {
	return layout.LibraryRoot + "/document-venv"
}

func (layout CompanyHostLayout) DocumentRequirementsPath() string {
	return layout.LibraryRoot + "/document-conversion/requirements.txt"
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

// ACPSocketDirectoryPath holds the one socket blueclaw serves the relay on, and
// nothing else. It is blueclaw's, setgid with the relay's group, so the socket
// blueclaw creates in it takes that group and the relay connects through the
// group bits without being given the blueclaw group, which reads the keys.
func (layout CompanyHostLayout) ACPSocketDirectoryPath() string {
	return layout.RunPath + "/acp"
}

func (layout CompanyHostLayout) ACPSocketPath() string {
	return layout.ACPSocketDirectoryPath() + "/blueclaw-acp.sock"
}

// AdmindSocketPath is where admind answers the relay and the capability daemon
// with an asserted requester. admind hands the socket to the relay's account.
func (layout CompanyHostLayout) AdmindSocketPath() string {
	return layout.RunPath + "/admind.sock"
}

func (layout CompanyHostLayout) RunSecretsPath() string {
	return layout.RunPath + "/secrets"
}

func (layout CompanyHostLayout) RunAssertionKeyPath() string {
	return layout.RunSecretsPath() + "/blueclaw-assertion-key"
}

func (layout CompanyHostLayout) RunModelKeyPath() string {
	return layout.RunSecretsPath() + "/openrouter-key"
}

func (layout CompanyHostLayout) CapabilitySocketPath() string {
	return layout.RunPath + "/capability.sock"
}
