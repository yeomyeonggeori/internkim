package companyhost

import (
	"fmt"
	"io"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The Linux half, which is what the package installs onto and what
// tools/test-native-install judges. It is one platform for every distribution
// with systemd: the database and the cache are units this package ships and runs
// from the server binaries the distribution installed, so nothing here depends
// on what the distribution calls its own PostgreSQL or Redis service.

var linuxDataServiceUnits = map[string]string{
	databaseServiceName: blueclaw.CompanyHostDatabaseServiceName + ".service",
	cacheServiceName:    blueclaw.CompanyHostCacheServiceName + ".service",
}

type linuxPlatform struct{}

func (linuxPlatform) Describe() string {
	return "Linux"
}

func (linuxPlatform) NameOfItsSupervisor() string {
	return "systemd"
}

func (linuxPlatform) Layout() blueclaw.CompanyHostLayout {
	return blueclaw.LinuxCompanyHostLayout()
}

func (linuxPlatform) EnsureServiceAccounts(Machine) error {
	return nil
}

func (linuxPlatform) StartTheDatabaseAndTheCache(machine Machine) error {
	units := []string{linuxDataServiceUnits[databaseServiceName], linuxDataServiceUnits[cacheServiceName]}
	if errorValue := machine.Run("systemctl", append([]string{"enable", "--now"}, units...), nil, io.Discard); errorValue != nil {
		return fmt.Errorf(
			"%s could not be started. They are this package's own database and cache, run from the PostgreSQL and Redis or Valkey servers this machine installed, and the company server keeps everything it knows in them: %w",
			strings.Join(units, " and "), errorValue)
	}
	return nil
}

// The cluster authenticates its own superuser by peer, so the way in is to
// become that account. The password reaches psql through the environment and
// never through a command line, which every account on this box can read out of
// /proc.
func (platform linuxPlatform) RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error {
	arguments := []string{
		"-u", blueclaw.CompanyHostDatabaseUser, "--",
		"sh", "-c", `printf '%s' "$` + databasePreparationVariable + `" | ` + platform.Layout().DataServicePath() +
			` psql --set ON_ERROR_STOP=1 --quiet --dbname postgres`,
	}
	return machine.Run("runuser", arguments, []string{databasePreparationVariable + "=" + statements}, progress)
}

func (platform linuxPlatform) SuperviseTheBundle(machine Machine, progress io.Writer) error {
	names := []string{}
	restarted := []string{}
	for _, unit := range blueclaw.CompanyHostSystemdUnits(platform.Layout()) {
		names = append(names, unit.FileName())
		if !unit.IsDataService() {
			restarted = append(restarted, unit.FileName())
		}
	}
	if errorValue := machine.Run("systemctl", append([]string{"enable"}, names...), nil, io.Discard); errorValue != nil {
		return fmt.Errorf("the company server's services could not be enabled, so they would not come back after a restart: %w", errorValue)
	}
	if errorValue := machine.Run("systemctl", append([]string{"restart"}, restarted...), nil, progress); errorValue != nil {
		return fmt.Errorf("the company server's services could not be started: %w", errorValue)
	}
	return nil
}

// A person reads one command, not a list of twelve names to look up. The
// command is the one this machine's own package manager takes, with the names
// the declaration gives that manager, because a Debian package name is the wrong
// name on a machine that does not use Debian packages. A machine with none of
// the four managers the declaration names gets the pieces by what they are.
func (linuxPlatform) HowToInstallTheseByHand(machine Machine, missing []missingPiece) []string {
	manager, isKnown := packageManagerOf(machine)
	if !isKnown {
		return []string{
			"  The rest are missing and this machine has none of " + knownPackageManagerNames() + " to name them for:",
			"    " + strings.Join(sortedAndUnique(whatEachPieceIs(missing)), ", "),
			"  Install them however this machine installs software, then run this again.",
		}
	}
	names := []string{}
	for _, piece := range missing {
		names = append(names, blueclaw.HostPackagesToInstallFor(manager, piece.Dependency)...)
	}
	return []string{
		"  Your distribution carries the rest. Install them, then run this again:",
		"    sudo " + strings.Join(manager.InstallWords(), " ") + " " + strings.Join(sortedAndUnique(names), " "),
	}
}

func packageManagerOf(machine Machine) (blueclaw.PackageManager, bool) {
	for _, manager := range blueclaw.PackageManagers() {
		if machine.CarriesProgram(string(manager)) == nil {
			return manager, true
		}
	}
	return "", false
}

func knownPackageManagerNames() string {
	names := []string{}
	for _, manager := range blueclaw.PackageManagers() {
		names = append(names, string(manager))
	}
	return strings.Join(names, ", ")
}

func (linuxPlatform) CarriesItInThePackage(dependency blueclaw.HostDependency) bool {
	return dependency.WhatThePackageCarriesInstead != ""
}

// A Linux box looks for a dependency on PATH or at the path the declaration
// names.
func (linuxPlatform) WhereToLookFor(blueclaw.HostDependency) []string {
	return nil
}

func (linuxPlatform) SupervisorIdentityFor(serviceName string) string {
	if unit, isOurs := linuxDataServiceUnits[serviceName]; isOurs {
		return unit
	}
	return serviceName + ".service"
}

func (linuxPlatform) SupervisorStateOf(machine Machine, identity string) string {
	state, errorValue := machine.Output("systemctl", []string{"is-active", identity})
	if errorValue != nil && state == "" {
		return ""
	}
	return strings.TrimSpace(state)
}

func (linuxPlatform) HowToSeeWhyItIsSilent(identity string) []string {
	return []string{
		"  what it is doing:  systemctl status " + identity,
		"  why it is not:     journalctl -u " + identity + " -n 50 --no-pager",
	}
}
