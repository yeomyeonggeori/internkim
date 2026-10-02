package companyhost

import (
	"fmt"
	"io"
	"runtime"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The same install, on the two machines a company host runs on.
//
// Nothing here decides what the company host *is*. The services come from
// blueclaw.CompanyHostServices, the units and the plists are rendered from that
// one declaration, the dependency list is blueclaw.HostDependencies, and the
// readiness probes are the same list either way. What a platform supplies is how
// this machine makes an account, reaches its database, supervises a process, and
// names the command that installs something.

// The two services the company host runs from servers it does not ship. A probe
// names them by what they are, and each platform turns that into what its
// supervisor calls them: a unit under systemd, a label under launchd.
const (
	databaseServiceName = "database"
	cacheServiceName    = "cache"
)

var companyHostDataServiceNames = map[string]string{
	databaseServiceName: blueclaw.CompanyHostDatabaseServiceName,
	cacheServiceName:    blueclaw.CompanyHostCacheServiceName,
}

type companyHostPlatform interface {
	Describe() string
	NameOfItsSupervisor() string
	Layout() blueclaw.CompanyHostLayout
	// EnsureServiceAccounts creates the unprivileged accounts the bundle runs
	// services as, where the package did not declare them.
	EnsureServiceAccounts(machine Machine) error
	StartTheDatabaseAndTheCache(machine Machine) error
	RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error
	SuperviseTheBundle(machine Machine, progress io.Writer) error
	// CarriesItInThePackage is true for a dependency the package on this
	// machine brings with it, so the machine is not asked for it.
	CarriesItInThePackage(dependency blueclaw.HostDependency) bool
	// WhereToLookFor is where this machine keeps a dependency that is not on
	// PATH and not at the path the declaration names. Any one of them satisfies it.
	// Empty means look the way the declaration says.
	WhereToLookFor(dependency blueclaw.HostDependency) []string
	// WhereItKeepsTheProgram is the path of one of a dependency's programs on
	// this machine. Empty means look for it on PATH.
	WhereItKeepsTheProgram(dependency blueclaw.HostDependency, programName string) string
	// HowToInstallTheseByHand is the closing lines of the preflight refusal: the
	// one command this machine installs software with, or nothing when this
	// repository does not know it.
	HowToInstallTheseByHand(machine Machine, missing []missingPiece) []string
	SupervisorIdentityFor(serviceName string) string
	// SupervisorStateOf is the supervisor's own one-word answer, for a refusal
	// that has to say whether the process is even there.
	SupervisorStateOf(machine Machine, identity string) string
	// HowToSeeWhyItIsSilent is the two commands whose output explains it.
	HowToSeeWhyItIsSilent(identity string) []string
}

func ThisMachineKeepsABoxSessionFresh() bool {
	return runtime.GOOS == "linux"
}

func KeepTheBoxSessionFresh(machine Machine, progress io.Writer) error {
	unit := blueclaw.BoxServiceName + ".service"
	if errorValue := machine.Run("systemctl", []string{"enable", unit}, nil, io.Discard); errorValue != nil {
		return fmt.Errorf("the unit that keeps this computer's session fresh could not be enabled: %w", errorValue)
	}
	return machine.Run("systemctl", []string{"restart", unit}, nil, progress)
}

// ThisMachine is the platform the installer is running on. A machine this
// repository has no install for is refused by name rather than part way through.
func ThisMachine() (companyHostPlatform, error) {
	return platformFor(runtime.GOOS, homebrewPrefix())
}

func platformFor(operatingSystem string, homebrewPrefix string) (companyHostPlatform, error) {
	switch operatingSystem {
	case "linux":
		return linuxPlatform{}, nil
	case "darwin":
		if homebrewPrefix == "" {
			return nil, fmt.Errorf(
				"the company host on a Mac is installed by Homebrew and `brew --prefix` answered nothing. " +
					"Install Homebrew from https://brew.sh, then run this again")
		}
		return macPlatform{homebrewPrefix: homebrewPrefix, launchDaemonRoot: blueclaw.CompanyHostLaunchDaemonRoot}, nil
	}
	return nil, fmt.Errorf("the company host installs on Linux with systemd and on macOS; this machine is %s", operatingSystem)
}
