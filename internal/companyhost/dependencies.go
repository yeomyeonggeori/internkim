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

// The one program whose presence decides whether this repository knows the
// command that installs the rest.
const debianPackageManager = "apt-get"

type missingPiece struct {
	What          string
	DebianPackage string
}

func requireWhatTheCompanyHostRuns(machine Machine) error {
	missing := whatThisComputerIsMissing(machine)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s", refusalNaming(missing, machine.CarriesProgram(debianPackageManager) == nil))
}

func whatThisComputerIsMissing(machine Machine) []missingPiece {
	missing := []missingPiece{}
	for _, program := range blueclaw.HostProgramsThePackageShips() {
		if machine.CarriesProgram(program) != nil {
			missing = append(missing, missingPiece{What: program, DebianPackage: blueclaw.CompanyPackageName})
		}
	}
	for _, dependency := range blueclaw.HostDependencies() {
		for _, program := range dependency.ProgramsTheHostRuns {
			if machine.CarriesProgram(program) != nil {
				missing = append(missing, missingPiece{What: program, DebianPackage: whatCarries(dependency)})
			}
		}
		if dependency.ReadableFilePath == "" {
			continue
		}
		if machine.CarriesFile(dependency.ReadableFilePath) != nil {
			missing = append(missing, missingPiece{What: dependency.ReadableFilePath, DebianPackage: whatCarries(dependency)})
		}
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
func refusalNaming(missing []missingPiece, thisMachineHasApt bool) string {
	fromTheDistribution := []string{}
	fromOurPackage := []string{}
	for _, piece := range missing {
		if piece.DebianPackage == blueclaw.CompanyPackageName {
			fromOurPackage = append(fromOurPackage, piece.What)
			continue
		}
		if thisMachineHasApt {
			fromTheDistribution = append(fromTheDistribution, piece.DebianPackage)
			continue
		}
		fromTheDistribution = append(fromTheDistribution, piece.What)
	}
	lines := []string{"this computer is missing what the company server runs on:"}
	if len(fromOurPackage) > 0 {
		lines = append(lines,
			"  "+strings.Join(sortedAndUnique(fromOurPackage), ", ")+
				" belong to the "+blueclaw.CompanyPackageName+" package and are not on this machine.",
			"  Install it with: curl -fsSL https://intern.kim/install.sh | sh -s -- host")
	}
	if len(fromTheDistribution) == 0 {
		return strings.Join(lines, "\n")
	}
	if thisMachineHasApt {
		return strings.Join(append(lines,
			"  Your distribution carries the rest. Install them, then run this again:",
			"    sudo apt-get install "+strings.Join(sortedAndUnique(fromTheDistribution), " ")), "\n")
	}
	return strings.Join(append(lines,
		"  The rest are missing and this machine has no apt to name them for:",
		"    "+strings.Join(sortedAndUnique(fromTheDistribution), ", "),
		"  Install them however this machine installs software, then run this again."), "\n")
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
