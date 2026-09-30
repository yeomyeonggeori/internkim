package companyhost

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

type linuxPlatform struct {
	// root is prepended to every path this platform writes itself, which is
	// empty on a real machine and a temporary directory in a test.
	root string
}

func (platform linuxPlatform) onDisk(path string) string {
	return filepath.Join(platform.root, path)
}

func (linuxPlatform) Describe() string {
	return "Linux"
}

func (linuxPlatform) NameOfItsSupervisor() string {
	return "systemd"
}

func (linuxPlatform) Layout() blueclaw.CompanyHostLayout {
	return blueclaw.LinuxCompanyHostLayout()
}

// The package's maintainer script creates these accounts; a machine that took
// the unpackaged path has nobody to have done it. useradd and groupadd are
// shadow-utils and are on every distribution the package is for, where adduser
// is Debian's alone.
func (platform linuxPlatform) EnsureServiceAccounts(machine Machine) error {
	for _, account := range companyHostServiceAccounts(platform.Layout()) {
		if _, errorValue := machine.Output("getent", []string{"passwd", account.Name}); errorValue == nil {
			continue
		}
		machine.Run("groupadd", []string{"--system", account.Name}, nil, io.Discard)
		arguments := []string{"--system", "--gid", account.Name, "--shell", nologinShell(machine)}
		if account.HomePath != "" {
			arguments = append(arguments, "--home-dir", account.HomePath)
		} else {
			arguments = append(arguments, "--no-create-home")
		}
		machine.Run("useradd", append(arguments, account.Name), nil, io.Discard)
		if _, errorValue := machine.Output("getent", []string{"passwd", account.Name}); errorValue != nil {
			return fmt.Errorf("the %s account the company host runs a service as could not be created: %w", account.Name, errorValue)
		}
	}
	return nil
}

func nologinShell(machine Machine) string {
	for _, path := range []string{"/usr/sbin/nologin", "/sbin/nologin", "/usr/bin/nologin"} {
		if machine.CarriesFile(path) == nil {
			return path
		}
	}
	return "/bin/false"
}

func (platform linuxPlatform) StartTheDatabaseAndTheCache(machine Machine) error {
	layout := platform.Layout()
	if errorValue := writeMissingDataServiceScript(platform.onDisk(layout.DataServicePath())); errorValue != nil {
		return errorValue
	}
	if errorValue := writeMissingSystemdUnits(platform.onDisk(blueclaw.CompanyPackageUnitRoot), layout, io.Discard); errorValue != nil {
		return errorValue
	}
	if errorValue := machine.Run("systemctl", []string{"daemon-reload"}, nil, io.Discard); errorValue != nil {
		return fmt.Errorf("systemd would not reload its units; this package supervises the company server with systemd: %w", errorValue)
	}
	units := []string{linuxDataServiceUnits[databaseServiceName], linuxDataServiceUnits[cacheServiceName]}
	if errorValue := machine.Run("systemctl", append([]string{"enable", "--now"}, units...), nil, io.Discard); errorValue != nil {
		return fmt.Errorf(
			"%s could not be started. They are this package's own database and cache, run from the PostgreSQL and Redis or Valkey servers this machine installed, and the company server keeps everything it knows in them: %w",
			strings.Join(units, " and "), errorValue)
	}
	return nil
}

func writeMissingDataServiceScript(path string) error {
	if _, errorValue := os.Stat(path); errorValue == nil {
		return nil
	}
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, []byte(blueclaw.CompanyHostDataServiceScript()), blueclaw.CompanyHostDataServiceMode)
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
	if errorValue := writeMissingSystemdUnits(platform.onDisk(blueclaw.CompanyPackageUnitRoot), platform.Layout(), progress); errorValue != nil {
		return errorValue
	}
	if errorValue := machine.Run("systemctl", []string{"daemon-reload"}, nil, progress); errorValue != nil {
		return fmt.Errorf("systemd would not reload its units; this package supervises the company server with systemd: %w", errorValue)
	}
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

// A unit already on disk belongs to whatever put it there, which on a packaged
// box is the package manager: rewriting it would make `dpkg --verify`, `rpm -V`
// or `pacman -Qkk` report the package modified. What is missing is written from the same renderer the package built
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
