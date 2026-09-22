package companyhost

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The Debian half, which is what the package installs onto and what
// tools/test-native-install judges. Nothing in it changed when the Mac arrived
// except that it moved behind the interface.

// The one program whose presence decides whether this repository knows the
// command that installs the rest.
const debianPackageManager = "apt-get"

// The database and the cache are the distribution's units, not ours. Nothing we
// ship starts them, so the install does.
var debianDistributionUnits = map[string]string{
	databaseServiceName: "postgresql.service",
	cacheServiceName:    "redis-server.service",
}

type debianPlatform struct{}

func (debianPlatform) Describe() string {
	return "Debian"
}

func (debianPlatform) NameOfItsSupervisor() string {
	return "systemd"
}

func (debianPlatform) Layout() blueclaw.CompanyHostLayout {
	return blueclaw.DebianCompanyHostLayout()
}

// The package's postinst creates these two accounts; a machine that took the
// unpackaged path has nobody to have done it.
func (debianPlatform) EnsureServiceAccounts(machine Machine) error {
	for _, account := range companyHostServiceAccounts(blueclaw.DebianCompanyHostLayout()) {
		if _, errorValue := machine.Output("getent", []string{"passwd", account.Name}); errorValue == nil {
			continue
		}
		machine.Run("addgroup", []string{"--system", account.Name}, nil, io.Discard)
		arguments := []string{"--system", "--ingroup", account.Name, "--shell", "/usr/sbin/nologin"}
		if account.HomePath != "" {
			arguments = append(arguments, "--home", account.HomePath)
		}
		machine.Run("adduser", append(arguments, account.Name), nil, io.Discard)
		if _, errorValue := machine.Output("getent", []string{"passwd", account.Name}); errorValue != nil {
			return fmt.Errorf("the %s account the company host runs a service as could not be created: %w", account.Name, errorValue)
		}
	}
	return nil
}

func (debianPlatform) StartTheDatabaseAndTheCache(machine Machine) error {
	units := []string{"postgresql", "redis-server"}
	if errorValue := machine.Run("systemctl", append([]string{"enable", "--now"}, units...), nil, io.Discard); errorValue != nil {
		return fmt.Errorf(
			"%s could not be started. They are the distribution's own services and the company server keeps everything it knows in them: %w",
			strings.Join(units, " and "), errorValue)
	}
	return nil
}

// Debian's cluster runs as postgres and authenticates by peer, so the way in is
// to become that account. The password reaches psql through the environment and
// never through a command line, which every account on this box can read out of
// /proc.
func (debianPlatform) RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error {
	arguments := []string{
		"-u", "postgres", "--",
		"sh", "-c", `printf '%s' "$` + databasePreparationVariable + `" | psql --set ON_ERROR_STOP=1 --quiet --no-psqlrc --dbname postgres`,
	}
	return machine.Run("runuser", arguments, []string{databasePreparationVariable + "=" + statements}, progress)
}

func (platform debianPlatform) SuperviseTheBundle(machine Machine, progress io.Writer) error {
	if errorValue := writeMissingSystemdUnits(blueclaw.CompanyPackageUnitRoot, platform.Layout(), progress); errorValue != nil {
		return errorValue
	}
	if errorValue := machine.Run("systemctl", []string{"daemon-reload"}, nil, progress); errorValue != nil {
		return fmt.Errorf("systemd would not reload its units; this package supervises the company server with systemd: %w", errorValue)
	}
	names := []string{}
	for _, unit := range blueclaw.CompanyPackageUnits() {
		names = append(names, unit.FileName())
	}
	if errorValue := machine.Run("systemctl", append([]string{"enable"}, names...), nil, io.Discard); errorValue != nil {
		return fmt.Errorf("the company server's services could not be enabled, so they would not come back after a restart: %w", errorValue)
	}
	if errorValue := machine.Run("systemctl", append([]string{"restart"}, names...), nil, progress); errorValue != nil {
		return fmt.Errorf("the company server's services could not be started: %w", errorValue)
	}
	return nil
}

// A unit already on disk belongs to whatever put it there, which on a packaged
// box is dpkg: rewriting it would make `dpkg --verify` report the package
// modified. What is missing is written from the same renderer the package built
// from, which is the whole of the claim that the two paths supervise the company
// host identically.
func writeMissingSystemdUnits(unitRoot string, layout blueclaw.CompanyHostLayout, progress io.Writer) error {
	if errorValue := os.MkdirAll(unitRoot, 0o755); errorValue != nil {
		return errorValue
	}
	for _, unit := range blueclaw.CompanyHostSystemdUnits(layout) {
		path := filepath.Join(unitRoot, unit.FileName())
		if _, errorValue := os.Stat(path); errorValue == nil {
			continue
		}
		if errorValue := os.WriteFile(path, []byte(unit.Contents), 0o644); errorValue != nil {
			return fmt.Errorf("write the %s unit: %w", unit.Name, errorValue)
		}
		fmt.Fprintf(progress, "  wrote %s\n", path)
	}
	return nil
}

// A person reads one command, not a list of twelve names to look up. The command
// is offered only where this repository knows it is the right one: on a machine
// with apt, the names are Debian's and the line is apt's. Anywhere else the
// missing pieces are named by what they are, because a Debian package name is
// the wrong name on a machine that does not use Debian packages, and advice a
// person cannot follow is worse than no advice.
func (debianPlatform) HowToInstallTheseByHand(machine Machine, missing []missingPiece) []string {
	names := []string{}
	for _, piece := range missing {
		names = append(names, piece.DebianPackage)
	}
	if machine.CarriesProgram(debianPackageManager) != nil {
		return []string{
			"  The rest are missing and this machine has no apt to name them for:",
			"    " + strings.Join(sortedAndUnique(whatEachPieceIs(missing)), ", "),
			"  Install them however this machine installs software, then run this again.",
		}
	}
	return []string{
		"  Your distribution carries the rest. Install them, then run this again:",
		"    sudo apt-get install " + strings.Join(sortedAndUnique(names), " "),
	}
}

// A Debian box looks for a dependency where Debian puts it, which is what the
// declaration already says.
func (debianPlatform) WhereToLookFor(blueclaw.HostDependency) []string {
	return nil
}

func (debianPlatform) SupervisorIdentityFor(serviceName string) string {
	if unit, isDistributions := debianDistributionUnits[serviceName]; isDistributions {
		return unit
	}
	return serviceName + ".service"
}

func (debianPlatform) SupervisorStateOf(machine Machine, identity string) string {
	state, errorValue := machine.Output("systemctl", []string{"is-active", identity})
	if errorValue != nil && state == "" {
		return ""
	}
	return strings.TrimSpace(state)
}

func (debianPlatform) HowToSeeWhyItIsSilent(identity string) []string {
	return []string{
		"  what it is doing:  systemctl status " + identity,
		"  why it is not:     journalctl -u " + identity + " -n 50 --no-pager",
	}
}
