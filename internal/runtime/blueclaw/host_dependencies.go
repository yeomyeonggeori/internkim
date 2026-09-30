package blueclaw

import "strings"

// HostPart is who in the company host needs a dependency. The agent image
// carries what the entrypoint, the daemons and the bundled skills reach for; a
// native package carries those and the messenger's, the cache's and the
// database's as well, because on that path nothing else brings them.
type HostPart string

const (
	HostPartEntrypoint     HostPart = "entrypoint"
	HostPartAgent          HostPart = "agent"
	HostPartDocumentSkills HostPart = "documentSkills"
	HostPartMessenger      HostPart = "messenger"
	HostPartCache          HostPart = "cache"
	HostPartDatabase       HostPart = "database"
)

// HostDependency is one thing the company host needs and does not build. A
// dependency is not one string: Debian and Homebrew disagree on the name, some
// of it is programs the host calls rather than packages it installs, one of it
// is a font file nothing puts on PATH, and some of them no distribution
// carries at all.
type HostDependency struct {
	DebianPackage          string
	DebianMinimumVersion   string
	DebianCallsItEssential bool
	// WhatTheDebianPackageCarriesInstead is for a dependency the .deb does not
	// ask the distribution for because the package brings its own. The host
	// image is a container and still installs DebianPackage, so the name stays.
	WhatTheDebianPackageCarriesInstead string
	HomebrewFormula                    string
	ArrivesAsPayload                   bool
	ProgramsTheHostRuns                []string
	ReadableFilePath                   string
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
		DebianPackage:        "postgresql",
		DebianMinimumVersion: "14",
		HomebrewFormula:      "postgresql@17",
		NeededBy:             []HostPart{HostPartDatabase},
	},
	{
		DebianPackage:       "postgresql-contrib",
		WhatAnswersItOnAMac: "Homebrew's postgresql@17 carries contrib",
		NeededBy:            []HostPart{HostPartDatabase},
	},
	{
		DebianPackage:       "redis-server",
		HomebrewFormula:     "redis",
		ProgramsTheHostRuns: []string{"redis-server"},
		NeededBy:            []HostPart{HostPartCache},
	},
	{
		DebianPackage:       "git",
		HomebrewFormula:     "git",
		ProgramsTheHostRuns: []string{"git"},
		NeededBy:            []HostPart{HostPartMessenger},
	},
	{
		DebianPackage:   "openssl",
		HomebrewFormula: "openssl@3",
		NeededBy:        []HostPart{HostPartMessenger},
	},
	{
		DebianPackage:       "ca-certificates",
		WhatAnswersItOnAMac: "macOS keeps the trust store in the system keychain",
		NeededBy:            []HostPart{HostPartAgent, HostPartMessenger},
	},
	{
		DebianPackage:       "curl",
		WhatAnswersItOnAMac: "macOS ships curl",
		ProgramsTheHostRuns: []string{"curl"},
		NeededBy:            []HostPart{HostPartAgent, HostPartMessenger},
	},
	{
		DebianPackage:       "jq",
		HomebrewFormula:     "jq",
		ProgramsTheHostRuns: []string{"jq"},
		NeededBy:            []HostPart{HostPartAgent},
	},
	{
		DebianPackage:       "unzip",
		WhatAnswersItOnAMac: "macOS ships unzip",
		ProgramsTheHostRuns: []string{"unzip"},
		NeededBy:            []HostPart{HostPartAgent},
	},
	{
		DebianPackage:       "postgresql-client",
		HomebrewFormula:     "postgresql@17",
		ProgramsTheHostRuns: []string{"pg_isready"},
		NeededBy:            []HostPart{HostPartEntrypoint},
	},
	{
		DebianPackage:       "netcat-openbsd",
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
		DebianPackage:                      "python3",
		ProgramsTheHostRuns:                []string{"python3"},
		WhatTheDebianPackageCarriesInstead: "a pinned relocatable CPython, the one the Mac carries, so no distribution's python3 or minor version matters",
		// See company_host_mac_interpreter.go: Homebrew's python@3.13 bottle
		// for macOS 26 cannot load pyexpat on 26.1, so the keg carries a
		// pinned relocatable CPython instead of depending on one.
		WhatAnswersItOnAMac: "the package carries a pinned relocatable CPython, because Homebrew's python@3.13 cannot load pyexpat on macOS 26.1",
		NeededBy:            []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:                      "python3-venv",
		WhatTheDebianPackageCarriesInstead: "the interpreter the package carries has venv in it",
		WhatAnswersItOnAMac:                "the interpreter the package carries has venv in it",
		NeededBy:                           []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:   "libfontconfig1",
		HomebrewFormula: "fontconfig",
		NeededBy:        []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:                      "fonts-nanum",
		ReadableFilePath:                   "/usr/share/fonts/truetype/nanum/NanumGothic.ttf",
		WhatTheDebianPackageCarriesInstead: "NanumGothic, under its own license, in a directory fontconfig scans",
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
		NeededBy:            []HostPart{HostPartEntrypoint},
	},
	{
		ArrivesAsPayload:    true,
		WhatAnswersItOnAMac: "the package carries it",
		ProgramsTheHostRuns: []string{AgentBrowserName},
		NeededBy:            []HostPart{HostPartEntrypoint},
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

// HostDebianDependsLine is the whole host, as a .deb control field. What the
// package carries is left out: a name in this line is something every
// distribution has to spell the same way and keep patched.
func HostDebianDependsLine() string {
	named := []string{}
	for _, dependency := range hostDependencies {
		if !dependency.isInstalledByAPackageManager() || dependency.DebianCallsItEssential {
			continue
		}
		if dependency.WhatTheDebianPackageCarriesInstead != "" {
			continue
		}
		if dependency.DebianMinimumVersion == "" {
			named = append(named, dependency.DebianPackage)
			continue
		}
		named = append(named, dependency.DebianPackage+" (>= "+dependency.DebianMinimumVersion+")")
	}
	return strings.Join(named, ", ")
}

// HostHomebrewDependencies is every `depends_on` line of the formula, and only
// those. It is a shorter list than Debian's because Homebrew's PostgreSQL
// carries contrib, the keg carries its own Python, and macOS supplies curl,
// unzip, netcat and the CA bundle itself; none of it means the Mac needs less.
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
