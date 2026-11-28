package companyhost

import (
	"fmt"
	"io"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The Linux half, which is what the package installs onto and what
// tools/test-native-install judges. It is one platform for every distribution
// with systemd: the database and the cache are units this package ships and runs
// from the server binaries the distribution installed, so nothing here depends
// on what the distribution calls its own PostgreSQL or Redis service.

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

func (platform linuxPlatform) StartTheDatabaseAndTheCache(machine Machine) error {
	names := dataServiceNames()
	database := platform.SupervisorIdentityFor(blueclaw.CompanyHostDatabaseServiceName)
	errorValue := platform.EnableServices(machine, names)
	if errorValue == nil {
		errorValue = platform.StartServices(machine, names, io.Discard)
	}
	if errorValue != nil {
		return fmt.Errorf(
			"%s could not be started. They are this package's own database and cache, run from the PostgreSQL this package carries and the Redis or Valkey server this machine installed, and the company server keeps everything it knows in them: %w",
			strings.Join(platform.unitsNamed(names), " and "), errorValue)
	}
	if errorValue := machine.Run("systemctl", []string{"reload", database}, nil, io.Discard); errorValue != nil {
		return fmt.Errorf(
			"%s would not take this release's access rules, so the services that reach the database as root would be refused: %w",
			database, errorValue)
	}
	return nil
}

// The cluster authenticates its own superuser by peer, so the way in is to
// become that account. The password reaches psql through the environment and
// never through a command line, which every account on this box can read out of
// /proc.
func (platform linuxPlatform) RunDatabaseStatements(machine Machine, statements string, progress io.Writer) error {
	arguments := asAnotherAccount(blueclaw.CompanyHostDatabaseUser,
		"sh", "-c", `printf '%s' "$`+databasePreparationVariable+`" | `+platform.Layout().DataServicePath()+
			` psql --set ON_ERROR_STOP=1 --quiet --dbname postgres`,
	)
	return machine.Run("runuser", arguments, []string{databasePreparationVariable + "=" + statements}, progress)
}

const neutralWorkingDirectory = "/"

func asAnotherAccount(account string, command ...string) []string {
	return append([]string{"-u", account, "--", "env", "--chdir=" + neutralWorkingDirectory}, command...)
}

func (platform linuxPlatform) SuperviseTheBundle(machine Machine, progress io.Writer) error {
	restarted := platform.CompanyServiceNames()
	if errorValue := platform.EnableServices(machine, append(dataServiceNames(), restarted...)); errorValue != nil {
		return fmt.Errorf("the company server's services could not be enabled, so they would not come back after a restart: %w", errorValue)
	}
	if errorValue := platform.RestartServices(machine, restarted, progress); errorValue != nil {
		return fmt.Errorf("the company server's services could not be started: %w", errorValue)
	}
	return nil
}

func (platform linuxPlatform) CompanyServiceNames() []string {
	names := []string{}
	for _, unit := range blueclaw.CompanyHostSystemdUnits(platform.Layout()) {
		if !unit.IsDataService() {
			names = append(names, unit.Name)
		}
	}
	return names
}

func (platform linuxPlatform) SuperviseTheBox(machine Machine, progress io.Writer) error {
	return superviseTheBoxService(platform, machine, progress)
}

func (linuxPlatform) BoxAbilities() BoxAbilities {
	return BoxAbilities{CanSetUpWifi: true, CanRenameItself: true, CanSetAdminPassword: true}
}

func (platform linuxPlatform) EnableServices(machine Machine, names []string) error {
	return platform.systemctl(machine, "enable", names, io.Discard)
}

func (platform linuxPlatform) DisableServices(machine Machine, names []string) error {
	return platform.systemctl(machine, "disable", names, io.Discard)
}

func (platform linuxPlatform) StartServices(machine Machine, names []string, progress io.Writer) error {
	return platform.systemctl(machine, "start", names, progress)
}

func (platform linuxPlatform) StopServices(machine Machine, names []string, progress io.Writer) error {
	return platform.systemctl(machine, "stop", names, progress)
}

func (platform linuxPlatform) RestartServices(machine Machine, names []string, progress io.Writer) error {
	return platform.systemctl(machine, "restart", names, progress)
}

func (platform linuxPlatform) StateOfService(machine Machine, name string) ServiceState {
	return serviceStateFromSystemd(platform.SupervisorStateOf(machine, platform.SupervisorIdentityFor(name)))
}

func serviceStateFromSystemd(activeState string) ServiceState {
	switch activeState {
	case "active", "activating", "reloading", "refreshing", "deactivating":
		return ServiceRunning
	case "failed":
		return ServiceFailed
	}
	return ServiceStopped
}

func (platform linuxPlatform) systemctl(machine Machine, verb string, names []string, output io.Writer) error {
	if len(names) == 0 {
		return nil
	}
	units := platform.unitsNamed(names)
	if errorValue := machine.Run("systemctl", append([]string{verb}, units...), nil, output); errorValue != nil {
		return fmt.Errorf("systemctl %s %s: %w", verb, strings.Join(units, " "), errorValue)
	}
	return nil
}

func (platform linuxPlatform) unitsNamed(names []string) []string {
	units := []string{}
	for _, name := range names {
		units = append(units, platform.SupervisorIdentityFor(name))
	}
	return units
}

// A person reads one command, not a list of names to look up. The
// command is the one this machine's own package manager takes, with the names
// the declaration gives that manager, because a Debian package name is the wrong
// name on a machine that does not use Debian packages. A machine with none of
// the managers the declaration names gets the pieces by what they are.
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

func (linuxPlatform) WhereItKeepsTheProgram(blueclaw.HostDependency, string) string {
	return ""
}

func (linuxPlatform) SupervisorIdentityFor(serviceName string) string {
	if name, isDataService := companyHostDataServiceNames[serviceName]; isDataService {
		return name + ".service"
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
