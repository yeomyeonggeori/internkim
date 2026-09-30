package companyhost

import (
	"fmt"
	"io"
	"runtime"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The same install, on the two machines a company host runs on. Six steps, and
// only four of them differ: the company's files are written the same way on
// both, and the wait is the same six probes against the same loopback addresses.
//
// Nothing here decides what the company host *is*. The services come from
// blueclaw.CompanyHostServices, the units and the plists are rendered from that
// one declaration, the dependency list is blueclaw.HostDependencies, and the
// readiness probes are the same list either way. What a platform supplies is how
// this machine makes an account, reaches its database, supervises a process, and
// names the command that installs something.

// The two services the company host opens and does not ship. They are named here
// because each platform calls them something different — postgresql and
// redis-server on Debian, postgresql@17 and redis under `brew services` — and a
// probe should not have to know which machine it is on.
const (
	databaseServiceName = "database"
	cacheServiceName    = "cache"
)

type companyHostPlatform interface {
	// Describe is what a refusal calls this machine.
	Describe() string
	// NameOfItsSupervisor is what keeps the services running here.
	NameOfItsSupervisor() string
	// Layout is where this machine keeps the company host's own files.
	Layout() blueclaw.CompanyHostLayout
	// EnsureServiceAccounts creates the two unprivileged accounts the bundle
	// runs services as.
	EnsureServiceAccounts(machine Machine) error
	// StartTheDatabaseAndTheCache starts what the distribution supervises.
	StartTheDatabaseAndTheCache(machine Machine) error
	// RunDatabaseStatements runs SQL as somebody the cluster lets in.
	RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error
	// SuperviseTheBundle writes the service definitions, registers them and
	// starts them.
	SuperviseTheBundle(machine Machine, progress io.Writer) error
	// CarriesItInThePackage is true for a dependency the package on this
	// machine brings with it, so the machine is not asked for it.
	CarriesItInThePackage(dependency blueclaw.HostDependency) bool
	// WhereToLookFor is where this machine keeps a dependency that is not on
	// PATH and not where Debian puts it. Any one of the paths satisfies it.
	// Empty means look the way the declaration says.
	WhereToLookFor(dependency blueclaw.HostDependency) []string
	// HowToInstallTheseByHand is the closing lines of the preflight refusal: the
	// one command this machine installs software with, or nothing when this
	// repository does not know it.
	HowToInstallTheseByHand(machine Machine, missing []missingPiece) []string
	// SupervisorIdentityFor is what this machine's supervisor calls a service.
	SupervisorIdentityFor(serviceName string) string
	// SupervisorStateOf is the supervisor's own one-word answer, for a refusal
	// that has to say whether the process is even there.
	SupervisorStateOf(machine Machine, identity string) string
	// HowToSeeWhyItIsSilent is the two commands whose output explains it.
	HowToSeeWhyItIsSilent(identity string) []string
}

// ThisMachine is the platform the installer is running on. A machine this
// repository has no install for is refused by name rather than part way through.
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

func ThisMachine() (companyHostPlatform, error) {
	return platformFor(runtime.GOOS, homebrewPrefix())
}

// companyHostServiceAccount is one unprivileged account the bundle runs a
// service as. Two exist, and they are separate because the relay outlives the
// agent: it keeps answering when the agent is down, so it does not share the
// agent's identity or its files.
type companyHostServiceAccount struct {
	Name        string
	HomePath    string
	Description string
}

func companyHostServiceAccounts(layout blueclaw.CompanyHostLayout) []companyHostServiceAccount {
	accounts := []companyHostServiceAccount{
		{Name: blueclaw.BlueclawUser, HomePath: layout.AgentHomePath, Description: "internkim agent"},
		{Name: blueclaw.RelayUserName, Description: "internkim relay"},
	}
	if !layout.OwnsItsDataServices() {
		return accounts
	}
	return append(accounts,
		companyHostServiceAccount{Name: blueclaw.CompanyHostDatabaseUser, Description: "internkim database"},
		companyHostServiceAccount{Name: blueclaw.CompanyHostCacheUser, Description: "internkim cache"})
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
		return macPlatform{homebrewPrefix: homebrewPrefix}, nil
	}
	return nil, fmt.Errorf("the company host installs on Linux with systemd and on macOS; this machine is %s", operatingSystem)
}
