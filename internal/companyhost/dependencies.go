package companyhost

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The package expresses this list as its dependencies and the package manager
// satisfies it before a single file lands. A binary built from source and run
// on its own has no package manager holding it to anything, so the same
// declaration is read here and the gap is named instead of installed.

type missingPiece struct {
	What            string
	Dependency      blueclaw.HostDependency
	IsOurs          bool
	HomebrewFormula string
}

func requireWhatTheCompanyHostRuns(platform companyHostPlatform, machine Machine) error {
	missing := whatThisComputerIsMissing(platform, machine)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s", refusalNaming(platform, machine, missing))
}

// What is missing is asked of this machine in this machine's terms. A dependency
// that declares where it is on a Mac is looked for there rather than on PATH:
// the browser the deck renderer opens is an application bundle and the Hangul
// face is a file the system ships, and neither is a program a `command -v`
// would find.
func whatThisComputerIsMissing(platform companyHostPlatform, machine Machine) []missingPiece {
	missing := []missingPiece{}
	for _, program := range blueclaw.HostProgramsThePackageShips() {
		if !carriesWhatThePackageShips(platform, machine, program) {
			missing = append(missing, missingPiece{What: program, IsOurs: true})
		}
	}
	for _, dependency := range blueclaw.HostDependencies() {
		missing = append(missing, whatIsMissingOf(platform, machine, dependency)...)
	}
	return missing
}

func whatIsMissingOf(platform companyHostPlatform, machine Machine, dependency blueclaw.HostDependency) []missingPiece {
	describe := func(what string) missingPiece {
		return missingPiece{
			What:            what,
			Dependency:      dependency,
			IsOurs:          dependency.ArrivesAsPayload,
			HomebrewFormula: dependency.HomebrewFormula,
		}
	}
	if platform.CarriesItInThePackage(dependency) {
		return nil
	}
	if candidates := platform.WhereToLookFor(dependency); len(candidates) > 0 {
		for _, candidate := range candidates {
			if machine.CarriesFile(candidate) == nil {
				return nil
			}
		}
		return []missingPiece{describe(candidates[0])}
	}
	missing := []missingPiece{}
	if len(dependency.OneOfThesePrograms) > 0 && !carriesAnyOf(platform, machine, dependency) {
		missing = append(missing, describe(strings.Join(whereEachProgramIs(platform, dependency, dependency.OneOfThesePrograms), " or ")))
	}
	for _, program := range dependency.ProgramsTheHostRuns {
		if dependency.ArrivesAsPayload && !carriesWhatThePackageShips(platform, machine, program) {
			missing = append(missing, describe(program))
		}
		if !dependency.ArrivesAsPayload && !carriesTheProgram(platform, machine, dependency, program) {
			missing = append(missing, describe(whereEachProgramIs(platform, dependency, []string{program})[0]))
		}
	}
	if dependency.ReadableFilePath != "" && machine.CarriesFile(dependency.ReadableFilePath) != nil {
		missing = append(missing, describe(dependency.ReadableFilePath))
	}
	return missing
}

// A program the package ships is where the layout puts it, which on a Mac is
// the keg's libexec and never on PATH. PATH answers only for what the
// operating system or its package manager provides.
func carriesWhatThePackageShips(platform companyHostPlatform, machine Machine, program string) bool {
	return machine.CarriesFile(platform.Layout().BinaryPath(program)) == nil
}

func carriesAnyOf(platform companyHostPlatform, machine Machine, dependency blueclaw.HostDependency) bool {
	for _, program := range dependency.OneOfThesePrograms {
		if carriesTheProgram(platform, machine, dependency, program) {
			return true
		}
	}
	return false
}

func carriesTheProgram(platform companyHostPlatform, machine Machine, dependency blueclaw.HostDependency, program string) bool {
	if path := platform.WhereItKeepsTheProgram(dependency, program); path != "" {
		return machine.CarriesFile(path) == nil
	}
	return machine.CarriesProgram(program) == nil
}

func whereEachProgramIs(platform companyHostPlatform, dependency blueclaw.HostDependency, programs []string) []string {
	described := []string{}
	for _, program := range programs {
		if path := platform.WhereItKeepsTheProgram(dependency, program); path != "" {
			program = path
		}
		described = append(described, program)
	}
	return described
}

// Our own programs are one line, the install script. What the distribution
// carries is the platform's to phrase, in the words of the manager it has.
func refusalNaming(platform companyHostPlatform, machine Machine, missing []missingPiece) string {
	fromOurPackage := []string{}
	fromElsewhere := []missingPiece{}
	for _, piece := range missing {
		if piece.IsOurs {
			fromOurPackage = append(fromOurPackage, piece.What)
			continue
		}
		fromElsewhere = append(fromElsewhere, piece)
	}
	lines := []string{"this computer is missing what the company server runs on:"}
	if len(fromOurPackage) > 0 {
		lines = append(lines,
			"  "+strings.Join(sortedAndUnique(fromOurPackage), ", ")+
				" belong to the "+blueclaw.CompanyPackageName+" package and are not on this machine.",
			"  Install it with: curl -fsSL https://intern.kim/install.sh | sh -s -- host")
	}
	if len(fromElsewhere) == 0 {
		return strings.Join(lines, "\n")
	}
	return strings.Join(append(lines, platform.HowToInstallTheseByHand(machine, fromElsewhere)...), "\n")
}

func whatEachPieceIs(missing []missingPiece) []string {
	described := []string{}
	for _, piece := range missing {
		described = append(described, piece.What)
	}
	return described
}

func sortedAndUnique(values []string) []string {
	seen := map[string]bool{}
	unique := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}
