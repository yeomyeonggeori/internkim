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
	What          string
	DebianPackage string
}

func requireWhatTheCompanyHostRuns(machine Machine) error {
	missing := whatThisComputerIsMissing(machine)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%s", refusalNaming(missing))
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

// A person reads one command, not a list of twelve names to look up. What the
// distribution carries becomes one apt-get line; what only our own package
// carries cannot, and says so, because `apt-get install internkim` is what put
// this program on the machine in the first place.
func refusalNaming(missing []missingPiece) string {
	fromTheDistribution := []string{}
	fromOurPackage := []string{}
	for _, piece := range missing {
		if piece.DebianPackage == blueclaw.CompanyPackageName {
			fromOurPackage = append(fromOurPackage, piece.What)
			continue
		}
		fromTheDistribution = append(fromTheDistribution, piece.DebianPackage)
	}
	lines := []string{"this computer is missing what the company server runs on:"}
	if len(fromOurPackage) > 0 {
		lines = append(lines,
			"  "+strings.Join(sortedAndUnique(fromOurPackage), ", ")+
				" belong to the "+blueclaw.CompanyPackageName+" package and are not on this machine.",
			"  Install it with: curl -fsSL https://intern.kim/install.sh | sh -s -- host")
	}
	if len(fromTheDistribution) > 0 {
		lines = append(lines,
			"  Your distribution carries the rest. Install them, then run this again:",
			"    sudo apt-get install "+strings.Join(sortedAndUnique(fromTheDistribution), " "))
	}
	return strings.Join(lines, "\n")
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
