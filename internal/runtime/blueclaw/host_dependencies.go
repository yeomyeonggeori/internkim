package blueclaw

import (
	"strings"
)

// HostPart is who in the company host needs a dependency. The agent image
// carries what its entrypoint, the daemons and the bundled skills reach for; a
// native package carries all but the entrypoint's, and the messenger's, the
// cache's and the database's as well, because on that path nothing else brings
// them.
type HostPart string

const (
	HostPartEntrypoint     HostPart = "entrypoint"
	HostPartAgent          HostPart = "agent"
	HostPartDocumentSkills HostPart = "documentSkills"
	HostPartMessenger      HostPart = "messenger"
	HostPartCache          HostPart = "cache"
	HostPartDatabase       HostPart = "database"
)

// PackageManager is one of the tools that installs the host's dependencies on
// Linux. The value is the program that answers whether a machine has it.
type PackageManager string

const (
	PackageManagerApt    PackageManager = "apt-get"
	PackageManagerDnf    PackageManager = "dnf"
	PackageManagerPacman PackageManager = "pacman"
)

// InstallWords is what a person types to install packages with this manager,
// as words.
func (manager PackageManager) InstallWords() []string {
	switch manager {
	case PackageManagerDnf:
		return []string{"dnf", "install"}
	case PackageManagerPacman:
		return []string{"pacman", "-S", "--needed"}
	}
	return []string{"apt-get", "install"}
}

// PackageManagers is every manager the table names packages for, in the order
// install.sh looks for them.
func PackageManagers() []PackageManager {
	return []PackageManager{PackageManagerApt, PackageManagerDnf, PackageManagerPacman}
}

// HostGlibcMinimum is the oldest glibc the package runs on, and the one place
// the number is written. buzz-relay is built against it, and every package
// format states it as a dependency so an older machine is refused with the
// package manager's own message.
const HostGlibcMinimum = "2.35"

// HostDependency is one thing the company host needs and does not build. A
// dependency is not one string: Debian and Homebrew disagree on the name, some
// of it is programs the host calls rather than packages it installs, one of it
// is a font file nothing puts on PATH, and some of them no distribution
// carries at all.
type HostDependency struct {
	DebianPackage string
	// DebianAlternatives are other apt names that satisfy the dependency, in
	// the order apt should try them after DebianPackage.
	DebianAlternatives []string
	// DebianCallsItEssential is also true of every other family's base system
	// for the two packages that carry it, so no manager names them.
	DebianCallsItEssential bool
	// The other managers' names. Several names are alternatives, tried in
	// order; pacman has no syntax for them and names one.
	DnfPackages    []string
	PacmanPackages []string
	// WhatBringsItInstead is for a manager that installs this without being
	// told to: Arch ships contrib inside postgresql. A
	// manager with neither a name nor an entry here is a hole, and a test fails
	// on it.
	WhatBringsItInstead map[PackageManager]string
	// WhatThePackageCarriesInstead is for a dependency the native packages
	// do not ask the distribution for because the package brings its own. The
	// host image is a container and still installs DebianPackage, so the name
	// stays.
	WhatThePackageCarriesInstead string
	HomebrewFormula              string
	ArrivesAsPayload             bool
	ProgramsTheHostRuns          []string
	// OneOfThesePrograms is for a dependency any one of several programs
	// satisfies, such as the cache's server, which is valkey-server on some
	// distributions and redis-server on others.
	OneOfThesePrograms []string
	ReadableFilePath   string
	// MacFilePathCandidates are where this dependency is on a Mac when it is
	// neither on PATH nor where Debian puts it: an application bundle, a font
	// the system ships. Any one of them satisfies the dependency, and a machine
	// with none of them is missing it.
	MacFilePathCandidates []string
	// WhatAnswersItOnAMac is for a dependency that needs neither a formula, a
	// cask nor a path because the Mac already has it. It is a sentence rather
	// than a flag so that "macOS ships this" and "nobody has looked" cannot be
	// the same empty field.
	WhatAnswersItOnAMac string
	NeededBy            []HostPart
}

// HostReadableFile is a path whose absence the host reports by name, because
// nothing on PATH answers for it.
type HostReadableFile struct {
	Path          string
	DebianPackage string
}

var hostDependencies = []HostDependency{
	{
		DebianPackage:   "postgresql",
		DnfPackages:     []string{"postgresql-server"},
		PacmanPackages:  []string{"postgresql"},
		HomebrewFormula: "postgresql@17",
		NeededBy:        []HostPart{HostPartDatabase},
	},
	{
		DebianPackage: "postgresql-contrib",
		DnfPackages:   []string{"postgresql-contrib"},
		WhatBringsItInstead: map[PackageManager]string{
			PackageManagerApt:    "a Debian server package carries contrib",
			PackageManagerPacman: "Arch's postgresql package carries contrib",
		},
		WhatAnswersItOnAMac: "Homebrew's postgresql@17 carries contrib",
		NeededBy:            []HostPart{HostPartDatabase},
	},
	{
		DebianPackage:      "redis-server",
		DebianAlternatives: []string{"valkey-server"},
		DnfPackages:        []string{"valkey", "redis"},
		PacmanPackages:     []string{"valkey"},
		HomebrewFormula:    "redis",
		OneOfThesePrograms: []string{"valkey-server", "redis-server"},
		NeededBy:           []HostPart{HostPartCache},
	},
	{
		DebianPackage:       "git",
		DnfPackages:         []string{"git"},
		PacmanPackages:      []string{"git"},
		HomebrewFormula:     "git",
		ProgramsTheHostRuns: []string{"git"},
		NeededBy:            []HostPart{HostPartMessenger},
	},
	{
		DebianPackage:   "openssl",
		DnfPackages:     []string{"openssl"},
		PacmanPackages:  []string{"openssl"},
		HomebrewFormula: "openssl@3",
		NeededBy:        []HostPart{HostPartMessenger},
	},
	{
		DebianPackage:       "ca-certificates",
		DnfPackages:         []string{"ca-certificates"},
		PacmanPackages:      []string{"ca-certificates"},
		WhatAnswersItOnAMac: "macOS keeps the trust store in the system keychain",
		NeededBy:            []HostPart{HostPartAgent, HostPartMessenger},
	},
	{
		DebianPackage:       "curl",
		DnfPackages:         []string{"curl"},
		PacmanPackages:      []string{"curl"},
		WhatAnswersItOnAMac: "macOS ships curl",
		ProgramsTheHostRuns: []string{"curl"},
		NeededBy:            []HostPart{HostPartAgent, HostPartMessenger},
	},
	{
		DebianPackage:       "jq",
		DnfPackages:         []string{"jq"},
		PacmanPackages:      []string{"jq"},
		HomebrewFormula:     "jq",
		ProgramsTheHostRuns: []string{"jq"},
		NeededBy:            []HostPart{HostPartAgent},
	},
	{
		DebianPackage:       "postgresql-client",
		DnfPackages:         []string{"postgresql"},
		PacmanPackages:      []string{"postgresql"},
		HomebrewFormula:     "postgresql@17",
		ProgramsTheHostRuns: []string{"pg_isready"},
		NeededBy:            []HostPart{HostPartEntrypoint, HostPartDatabase},
	},
	{
		DebianPackage:       "netcat-openbsd",
		DnfPackages:         []string{"nmap-ncat"},
		PacmanPackages:      []string{"openbsd-netcat"},
		WhatAnswersItOnAMac: "macOS ships nc",
		ProgramsTheHostRuns: []string{"nc"},
		NeededBy:            []HostPart{HostPartEntrypoint},
	},
	{
		DebianPackage:          "coreutils",
		DebianCallsItEssential: true,
		WhatAnswersItOnAMac:    "BSD userland supplies all of them, and install(1) takes the same -d -o -g -m the preparation script uses",
		ProgramsTheHostRuns:    []string{"cat", "cp", "dirname", "install", "mkdir", "chown", "sleep"},
		NeededBy:               []HostPart{HostPartEntrypoint},
	},
	{
		DebianPackage:          "util-linux",
		DebianCallsItEssential: true,
		// setpriv belongs to host/entrypoint.sh, which is the container path the
		// package retires. Nothing the package installs runs it: a unit says
		// User= and a LaunchDaemon says UserName=.
		WhatAnswersItOnAMac: "nothing, and nothing needs to: setpriv is the container entrypoint's, and the supervisor drops privilege instead",
		ProgramsTheHostRuns: []string{"setpriv"},
		NeededBy:            []HostPart{HostPartEntrypoint},
	},
	{
		DebianPackage:       "python3",
		DnfPackages:         []string{"python3"},
		PacmanPackages:      []string{"python"},
		ProgramsTheHostRuns: []string{"python3"},
		WhatAnswersItOnAMac: "the Command Line Tools Homebrew requires put python3 in /usr/bin",
		NeededBy:            []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:   "libfontconfig1",
		DnfPackages:     []string{"fontconfig"},
		PacmanPackages:  []string{"fontconfig"},
		HomebrewFormula: "fontconfig",
		NeededBy:        []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:                "fonts-nanum",
		ReadableFilePath:             "/usr/share/fonts/truetype/nanum/NanumGothic.ttf",
		WhatThePackageCarriesInstead: "NanumGothic, under its own license, in a directory fontconfig scans",
		// Every Mac ships a Hangul face, and the skills that embed one already
		// accept it: pdf and paperwork both list AppleSDGothicNeo in their
		// requires-any-file declarations. So the cask is a nicety on macOS
		// rather than a dependency, and the formula neither names it nor
		// recommends it.
		MacFilePathCandidates: []string{"/System/Library/Fonts/AppleSDGothicNeo.ttc"},
		NeededBy:              []HostPart{HostPartDocumentSkills},
	},
	{
		ArrivesAsPayload:    true,
		WhatAnswersItOnAMac: "the package carries it",
		ProgramsTheHostRuns: []string{BunProgramName},
		NeededBy:            []HostPart{HostPartAgent, HostPartDocumentSkills},
	},
	{
		ArrivesAsPayload:    true,
		WhatAnswersItOnAMac: "the package carries it",
		ProgramsTheHostRuns: []string{PackageResolverName},
		NeededBy:            []HostPart{HostPartDocumentSkills},
	},
	{
		ArrivesAsPayload:    true,
		WhatAnswersItOnAMac: "the package carries it",
		ProgramsTheHostRuns: []string{DeviceBrowserName},
		NeededBy:            []HostPart{HostPartEntrypoint, HostPartAgent},
	},
	{
		ArrivesAsPayload:    true,
		WhatAnswersItOnAMac: "the package carries it",
		ProgramsTheHostRuns: []string{AgentBrowserName},
		NeededBy:            []HostPart{HostPartEntrypoint, HostPartAgent},
	},
	{
		ArrivesAsPayload:    true,
		WhatAnswersItOnAMac: "the package carries it",
		ProgramsTheHostRuns: []string{BuzzMediaProgramName},
		NeededBy:            []HostPart{HostPartMessenger},
	},
}

// HostDependencies is the company host's dependency list, and the only one.
func HostDependencies() []HostDependency {
	return append([]HostDependency(nil), hostDependencies...)
}

func (dependency HostDependency) neededByAnyOf(parts []HostPart) bool {
	for _, wanted := range parts {
		for _, declared := range dependency.NeededBy {
			if declared == wanted {
				return true
			}
		}
	}
	return false
}

func (dependency HostDependency) isInstalledByAPackageManager() bool {
	return dependency.DebianPackage != "" && !dependency.ArrivesAsPayload
}

// OnlyTheImageEntrypointRuns is a dependency of host/entrypoint.sh alone. A
// native host is supervised by systemd or launchd and runs no such script, so
// neither its package nor `internkim install` asks for one.
func (dependency HostDependency) OnlyTheImageEntrypointRuns() bool {
	return len(dependency.NeededBy) == 1 && dependency.NeededBy[0] == HostPartEntrypoint
}

// HostDebianPackagesFor names what apt-get installs for the parts asked about.
// Essential packages are left out: Debian guarantees them, and naming one in a
// dependency list is noise a reader has to re-derive.
func HostDebianPackagesFor(parts ...HostPart) []string {
	packages := []string{}
	for _, dependency := range hostDependencies {
		if !dependency.isInstalledByAPackageManager() || dependency.DebianCallsItEssential {
			continue
		}
		if dependency.neededByAnyOf(parts) {
			packages = append(packages, dependency.DebianPackage)
		}
	}
	return packages
}

// HostImageDebianPackages is what host/Dockerfile installs: the agent image
// reaches its database, cache and messenger over the network rather than
// carrying them.
func HostImageDebianPackages() []string {
	return HostDebianPackagesFor(HostPartEntrypoint, HostPartAgent, HostPartDocumentSkills)
}

// PackagesFor is every name the manager accepts for this dependency, in the
// order it should try them. It is empty when the manager needs to be told
// nothing, which IsNamedIn separates from a hole in the table.
func (dependency HostDependency) PackagesFor(manager PackageManager) []string {
	switch manager {
	case PackageManagerApt:
		return append([]string{dependency.DebianPackage}, dependency.DebianAlternatives...)
	case PackageManagerDnf:
		return dependency.DnfPackages
	case PackageManagerPacman:
		return dependency.PacmanPackages
	}
	return nil
}

// IsNamedIn is whether a native package's dependency list names this
// dependency for the manager. What the package carries, what the base system
// guarantees and what another row already pulls in are left out: a name in the
// list is something every distribution has to spell the same way and keep
// patched.
func (dependency HostDependency) IsNamedIn(manager PackageManager) bool {
	if !dependency.isInstalledByAPackageManager() || dependency.DebianCallsItEssential || dependency.OnlyTheImageEntrypointRuns() {
		return false
	}
	if dependency.WhatThePackageCarriesInstead != "" {
		return false
	}
	return dependency.WhatBringsItInstead[manager] == ""
}

// HostPackageDependsFor is the whole host as one manager's dependency list.
// A glibc floor comes first: it is what refuses a machine the binaries cannot
// run on before any of the names are tried.
func HostPackageDependsFor(manager PackageManager) []string {
	depends := []string{glibcDependencyFor(manager)}
	for _, dependency := range hostDependencies {
		if !dependency.IsNamedIn(manager) {
			continue
		}
		depends = appendOnce(depends, dependencyExpression(manager, dependency.PackagesFor(manager)))
	}
	return depends
}

func glibcDependencyFor(manager PackageManager) string {
	switch manager {
	case PackageManagerApt:
		return "libc6 (>= " + HostGlibcMinimum + ")"
	case PackageManagerPacman:
		return "glibc>=" + HostGlibcMinimum
	}
	return "glibc >= " + HostGlibcMinimum
}

// dependencyExpression is alternatives in the manager's own syntax. pacman has
// none, and a table row that gives it several is a test failure rather than a
// silent first choice.
func dependencyExpression(manager PackageManager, names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	if manager == PackageManagerApt {
		return strings.Join(names, " | ")
	}
	return "(" + strings.Join(names, " or ") + ")"
}

func appendOnce(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// HostPackagesToInstallFor is what a person types after `install` to get what
// the host needs on this manager: the first name of each alternative, for the
// rows the package would not bring on its own.
func HostPackagesToInstallFor(manager PackageManager, dependency HostDependency) []string {
	if !dependency.IsNamedIn(manager) {
		return nil
	}
	return dependency.PackagesFor(manager)[:1]
}

// HostHomebrewDependencies is every `depends_on` line of the formula, and only
// those. It is a shorter list than Debian's because Homebrew's PostgreSQL
// carries contrib and macOS supplies python3, curl and the CA bundle itself;
// none of it means the Mac needs less.
func HostHomebrewDependencies() []string {
	formulas := []string{}
	named := map[string]bool{}
	for _, dependency := range hostDependencies {
		if dependency.HomebrewFormula == "" || named[dependency.HomebrewFormula] {
			continue
		}
		named[dependency.HomebrewFormula] = true
		formulas = append(formulas, dependency.HomebrewFormula)
	}
	return formulas
}

func hostProgramsNeededBy(parts ...HostPart) []string {
	programs := []string{}
	for _, dependency := range hostDependencies {
		if dependency.neededByAnyOf(parts) {
			programs = append(programs, dependency.ProgramsTheHostRuns...)
		}
	}
	return programs
}

// HostProgramsThatArriveAsPayload names every program the package carries because no
// package manager has one. host_payload_downloads.go pins each of them, and
// TestThePinsCoverExactlyThePayloadProgramsDeclared reads both lists so that a program
// added here and nowhere else fails at build time rather than at a person's install.
func HostProgramsThatArriveAsPayload() []string {
	names := []string{}
	for _, dependency := range hostDependencies {
		if !dependency.ArrivesAsPayload {
			continue
		}
		names = append(names, dependency.ProgramsTheHostRuns...)
	}
	return names
}

// HostProgramsThePackageShips are the company host's own binaries, which the
// entrypoint checks for beside everything it did not build.
func HostProgramsThePackageShips() []string {
	return []string{
		CapabilitydName,
		AdmindName,
		MaildName,
		BlueclawName,
		ChatdName,
		RelayName,
		RenderCompanyRuntimeName,
	}
}

// ProgramsTheHostEntrypointRuns is every name host/entrypoint.sh invokes, so a
// box missing one is refused at the door rather than partway through bring-up.
func ProgramsTheHostEntrypointRuns() []string {
	return append(HostProgramsThePackageShips(), hostProgramsNeededBy(HostPartEntrypoint)...)
}

// ProgramsTheBundledSkillsRun is what the skills that write documents and decks
// reach for through the requester's shell. Their absence withholds a skill
// rather than stopping the box, which is why it is a separate answer.
func ProgramsTheBundledSkillsRun() []string {
	return hostProgramsNeededBy(HostPartDocumentSkills)
}

// HostFilesTheBundledSkillsRead are the paths those skills open directly, whose
// absence produces a plausible file rather than an error.
func HostFilesTheBundledSkillsRead() []HostReadableFile {
	files := []HostReadableFile{}
	for _, dependency := range hostDependencies {
		if dependency.ReadableFilePath == "" || !dependency.neededByAnyOf([]HostPart{HostPartDocumentSkills}) {
			continue
		}
		files = append(files, HostReadableFile{
			Path:          dependency.ReadableFilePath,
			DebianPackage: dependency.DebianPackage,
		})
	}
	return files
}

// BuzzRelayDatabasePackages is what the device path installs before it starts
// the relay bound to it.
func BuzzRelayDatabasePackages() string {
	return strings.Join(HostDebianPackagesFor(HostPartDatabase), " ")
}

// BuzzRelayCachePackages is what it installs before the relay that queues
// through it.
func BuzzRelayCachePackages() string {
	return strings.Join(HostDebianPackagesFor(HostPartCache), " ")
}
