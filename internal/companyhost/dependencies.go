package companyhost

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The package expresses this list as `Depends:` and apt satisfies it before a
// single file lands. Nothing else can: a machine that took the unpackaged path
// has no package manager holding it to anything, so the same declaration is
// read here and the gap is named instead of installed. The gap between the two
// paths is this message.

type missingPiece struct {
	What            string
	DebianPackage   string
	HomebrewFormula string
	HomebrewCask    blueclaw.HostHomebrewCask
}

func (piece missingPiece) belongsToOurPackage() bool {
	return piece.DebianPackage == blueclaw.CompanyPackageName
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
		if machine.CarriesProgram(program) != nil {
			missing = append(missing, missingPiece{What: program, DebianPackage: blueclaw.CompanyPackageName})
		}
	}
	for _, dependency := range blueclaw.HostDependencies() {
		for _, piece := range whatIsMissingOf(platform, machine, dependency) {
			missing = append(missing, piece)
		}
	}
	return missing
}

func whatIsMissingOf(platform companyHostPlatform, machine Machine, dependency blueclaw.HostDependency) []missingPiece {
	describe := func(what string) missingPiece {
		return missingPiece{
			What:            what,
			DebianPackage:   whatCarries(dependency),
			HomebrewFormula: dependency.HomebrewFormula,
			HomebrewCask:    dependency.HomebrewCask,
		}
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
	for _, program := range dependency.ProgramsTheHostRuns {
		if machine.CarriesProgram(program) != nil {
			missing = append(missing, describe(program))
		}
	}
	if dependency.ReadableFilePath != "" && machine.CarriesFile(dependency.ReadableFilePath) != nil {
		missing = append(missing, describe(dependency.ReadableFilePath))
	}
	return missing
}

func whatCarries(dependency blueclaw.HostDependency) string {
	if dependency.ArrivesAsPayload {
		return blueclaw.CompanyPackageName
	}
	return dependency.DebianPackage
}

// A person reads one command, not a list of twelve names to look up. The command
// is offered only where this repository knows it is the right one: on a machine
// with apt, the names are Debian's and the line is apt's. Anywhere else the
// missing pieces are named by what they are, because a Debian package name is
// the wrong name on a machine that does not use Debian packages, and advice a
// person cannot follow is worse than no advice.
func refusalNaming(platform companyHostPlatform, machine Machine, missing []missingPiece) string {
	fromOurPackage := []string{}
	fromElsewhere := []missingPiece{}
	for _, piece := range missing {
		if piece.belongsToOurPackage() {
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
