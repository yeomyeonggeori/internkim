package blueclaw

import (
	"fmt"
	"strconv"
	"strings"
)

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
// is a font file nothing puts on PATH, and four of them no distribution
// carries at all.
type HostDependency struct {
	DebianPackage          string
	DebianMinimumVersion   string
	DebianCallsItEssential bool
	HomebrewFormula        string
	HomebrewCask           HostHomebrewCask
	ArrivesAsPayload       bool
	ProgramsTheHostRuns    []string
	ReadableFilePath       string
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

// HostHomebrewCask is a dependency a formula cannot declare. Homebrew's
// DependencyCollector#parse_symbol_spec accepts :arch, :linux, :macos,
// :maximum_macos and :xcode and raises "Unsupported special dependency" on
// anything else; `cask:` is a key of the cask DSL's own depends_on. So a cask
// never becomes a depends_on line, and what the formula can do instead is say in
// its caveats what the person has to type.
type HostHomebrewCask struct {
	Name string
	// IsDisabledUpstream means `brew install --cask <Name>` refuses. Naming it
	// in a caveat would send a person at a command that fails, so the caveat
	// names InsteadInstall instead and says why.
	IsDisabledUpstream bool
	// WhyItIsDisabled is upstream's own reason, so the caveat and the report do
	// not have to guess at it.
	WhyItIsDisabled string
	// InsteadInstall is the cask that does work, when one does. Empty means
	// nothing on Homebrew answers for this and the dependency is met another
	// way or not at all.
	InsteadInstall string
	// WhatItIsFor is the sentence the caveat prints, so a person knows what
	// declining it costs.
	WhatItIsFor string
}

// IsDeclared distinguishes a dependency that names a cask from one that does not.
func (cask HostHomebrewCask) IsDeclared() bool {
	return cask.Name != ""
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
		DebianPackage:       "python3",
		ProgramsTheHostRuns: []string{"python3"},
		// See company_host_mac_interpreter.go: Homebrew's python@3.13 bottle
		// for macOS 26 cannot load pyexpat on 26.1, so the keg carries a
		// pinned relocatable CPython instead of depending on one.
		WhatAnswersItOnAMac: "the package carries a pinned relocatable CPython, because Homebrew's python@3.13 cannot load pyexpat on macOS 26.1",
		NeededBy:            []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:       "python3-venv",
		WhatAnswersItOnAMac: "the interpreter the package carries has venv in it",
		NeededBy:            []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:   "libfontconfig1",
		HomebrewFormula: "fontconfig",
		NeededBy:        []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:    "fonts-nanum",
		ReadableFilePath: "/usr/share/fonts/truetype/nanum/NanumGothic.ttf",
		// Every Mac ships a Hangul face, and the skills that embed one already
		// accept it: pdf and paperwork both list AppleSDGothicNeo in their
		// requires-any-file declarations. So the cask is a nicety on macOS
		// rather than a dependency, and the formula neither names it nor
		// recommends it.
		MacFilePathCandidates: []string{"/System/Library/Fonts/AppleSDGothicNeo.ttc"},
		NeededBy:              []HostPart{HostPartDocumentSkills},
	},
	{
		DebianPackage:       "chromium",
		ProgramsTheHostRuns: []string{"chromium"},
		// There is no `brew install` that puts a Chromium on a Mac. The cask was
		// disabled upstream on 2026-09-01 and `brew info --cask chromium` says
		// so. What the deck renderer opens instead is the browser it already
		// falls through to, which is a live cask and an application bundle
		// rather than anything on PATH.
		HomebrewCask: HostHomebrewCask{
			Name:               "chromium",
			IsDisabledUpstream: true,
			WhyItIsDisabled:    "it does not pass the macOS Gatekeeper check (disabled upstream on 2026-09-01)",
			InsteadInstall:     "google-chrome",
			WhatItIsFor:        "the skills that render slides and print documents drive a browser; without one they are withheld",
		},
		MacFilePathCandidates: []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		},
		NeededBy: []HostPart{HostPartDocumentSkills},
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

// DocumentInterpreterPackage is the distribution's python3, which is also the
// interpreter the package's own document venv is resolved against.
const DocumentInterpreterPackage = "python3"

// HostDebianDependsLine is the whole host, as a .deb control field.
//
// documentInterpreterVersion is the full version the package's document venv
// reports, and it bounds python3 to that minor version alone: the venv's
// site-packages are wheels built for one operating system, one processor and
// one Python minor version, and an interpreter outside the bound imports none
// of them. An empty version leaves python3 unbounded, which is what a path that
// ships no venv wants.
func HostDebianDependsLine(documentInterpreterVersion string) string {
	minimum, below, isBounded := documentInterpreterBound(documentInterpreterVersion)
	constrained := []string{}
	for _, dependency := range hostDependencies {
		if !dependency.isInstalledByAPackageManager() || dependency.DebianCallsItEssential {
			continue
		}
		if dependency.DebianPackage == DocumentInterpreterPackage && isBounded {
			constrained = append(constrained,
				DocumentInterpreterPackage+" (>= "+minimum+")",
				DocumentInterpreterPackage+" (<< "+below+")")
			continue
		}
		if dependency.DebianMinimumVersion == "" {
			constrained = append(constrained, dependency.DebianPackage)
			continue
		}
		constrained = append(constrained,
			dependency.DebianPackage+" (>= "+dependency.DebianMinimumVersion+")")
	}
	return strings.Join(constrained, ", ")
}

// documentInterpreterBound reads 3.13.5 as "3.13 or newer, below 3.14". The
// version is the venv's own answer rather than a literal anybody maintains, so
// the package cannot claim a python3 it was not resolved against.
func documentInterpreterBound(version string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) < 2 {
		return "", "", false
	}
	major, errorValue := strconv.Atoi(parts[0])
	if errorValue != nil {
		return "", "", false
	}
	minor, errorValue := strconv.Atoi(parts[1])
	if errorValue != nil {
		return "", "", false
	}
	return fmt.Sprintf("%d.%d", major, minor), fmt.Sprintf("%d.%d", major, minor+1), true
}

// HostHomebrewDependencies is every `depends_on` line of the formula, and only
// those. It is a shorter list than Debian's for three separate reasons, and none
// of them is that the Mac needs less: Homebrew's PostgreSQL carries contrib and
// its Python carries venv, macOS supplies curl, unzip, netcat and the CA
// bundle itself, and two of Debian's dependencies are casks a formula is
// forbidden to name. What the formula does about those two is
// HostHomebrewCasksAPersonMustInstall.
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

// HostHomebrewCasksAPersonMustInstall is what the formula's caveats say, because
// a formula cannot say it as a dependency. Each one names what it is for, so a
// person can decide to do without it and know what they gave up.
func HostHomebrewCasksAPersonMustInstall() []HostHomebrewCask {
	casks := []HostHomebrewCask{}
	for _, dependency := range hostDependencies {
		if !dependency.HomebrewCask.IsDeclared() {
			continue
		}
		casks = append(casks, dependency.HomebrewCask)
	}
	return casks
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
