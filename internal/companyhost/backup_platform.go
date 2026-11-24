package companyhost

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type backupPlatform interface {
	companyHostPlatform
	DatabaseMajor(machine Machine) (int, error)
	DumpDatabase(machine Machine, database string, output io.Writer) error
	RestoreDatabase(machine Machine, database string, input io.Reader, listPath string) error
	ListDump(machine Machine, input io.Reader) (string, error)
	OwnersInDump(machine Machine, input io.Reader) ([]string, error)
	StopTheHost(machine Machine, progress io.Writer) error
	StartTheBox(machine Machine, progress io.Writer) error
}

func backupPlatformOf(platform companyHostPlatform) (backupPlatform, error) {
	backing, isBackedUp := platform.(backupPlatform)
	if !isBackedUp {
		return nil, fmt.Errorf(
			"backup and restore run on a Linux host, and this computer runs %s",
			platform.Describe())
	}
	return backing, nil
}

func (platform linuxPlatform) asTheDatabaseAccount(arguments ...string) []string {
	return asAnotherAccount(blueclaw.CompanyHostDatabaseUser, append([]string{platform.Layout().DataServicePath()}, arguments...)...)
}

func (platform linuxPlatform) DatabaseMajor(machine Machine) (int, error) {
	answer, errorValue := machine.Output("runuser", platform.asTheDatabaseAccount("postgres-major"))
	if errorValue != nil {
		return 0, fmt.Errorf("the host's PostgreSQL would not say which major version it runs: %w", errorValue)
	}
	major, errorValue := strconv.Atoi(strings.TrimSpace(answer))
	if errorValue != nil {
		return 0, fmt.Errorf("the host's PostgreSQL answered %q when asked for its major version", strings.TrimSpace(answer))
	}
	return major, nil
}

func (platform linuxPlatform) DumpDatabase(machine Machine, database string, output io.Writer) error {
	var complaint bytes.Buffer
	arguments := platform.asTheDatabaseAccount("pg_dump", "--format", "custom", "--dbname", database)
	if errorValue := machine.Stream("runuser", arguments, Streams{Output: output, Errors: &complaint}); errorValue != nil {
		return fmt.Errorf("pg_dump of the %s database failed (%w): %s", database, errorValue, strings.TrimSpace(complaint.String()))
	}
	return nil
}

func (platform linuxPlatform) RestoreDatabase(machine Machine, database string, input io.Reader, listPath string) error {
	var complaint bytes.Buffer
	arguments := platform.asTheDatabaseAccount(
		"pg_restore", "--dbname", database, "--single-transaction", "--exit-on-error", "--no-password")
	if listPath != "" {
		arguments = append(arguments, "--use-list", listPath)
	}
	if errorValue := machine.Stream("runuser", arguments, Streams{Input: input, Output: &complaint, Errors: &complaint}); errorValue != nil {
		return fmt.Errorf("pg_restore into the %s database failed (%w): %s", database, errorValue, strings.TrimSpace(complaint.String()))
	}
	return nil
}

func (platform linuxPlatform) ListDump(machine Machine, input io.Reader) (string, error) {
	var list, complaint bytes.Buffer
	arguments := platform.asTheDatabaseAccount("pg_restore", "--list")
	if errorValue := machine.Stream("runuser", arguments, Streams{Input: input, Output: &list, Errors: &complaint}); errorValue != nil {
		return "", fmt.Errorf("pg_restore could not list the dump's contents (%w): %s", errorValue, strings.TrimSpace(complaint.String()))
	}
	return list.String(), nil
}

func (platform linuxPlatform) OwnersInDump(machine Machine, input io.Reader) ([]string, error) {
	script, writer := io.Pipe()
	var complaint bytes.Buffer
	arguments := platform.asTheDatabaseAccount("pg_restore", "--schema-only", "--file", "-")
	go func() {
		writer.CloseWithError(machine.Stream("runuser", arguments, Streams{Input: input, Output: writer, Errors: &complaint}))
	}()
	owners, errorValue := foreignOwners(script)
	io.Copy(io.Discard, script)
	if errorValue != nil {
		return nil, fmt.Errorf("pg_restore could not list the dump's owners (%w): %s", errorValue, strings.TrimSpace(complaint.String()))
	}
	return owners, nil
}

func (platform linuxPlatform) StopTheHost(machine Machine, progress io.Writer) error {
	units := append([]string{blueclaw.BoxServiceName + ".service"}, platform.companyServiceUnits()...)
	if errorValue := machine.Run("systemctl", append([]string{"stop"}, units...), nil, progress); errorValue != nil {
		return fmt.Errorf("the host's services could not be stopped, so nothing was restored: %w", errorValue)
	}
	return nil
}

func (platform linuxPlatform) companyServiceUnits() []string {
	units := []string{}
	for _, unit := range blueclaw.CompanyHostSystemdUnits(platform.Layout()) {
		if !unit.IsDataService() {
			units = append(units, unit.FileName())
		}
	}
	return units
}

func StandDown(machine Machine, progress io.Writer) error {
	if errorValue := StopTheServices(machine, progress); errorValue != nil {
		return errorValue
	}
	return forgetTheCurrentCompany()
}

func StopTheServices(machine Machine, progress io.Writer) error {
	linux, errorValue := thisLinuxBox("stops its services")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := disableNow(machine, linux.companyServiceUnits(), progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be stopped on this box: %w", errorValue)
	}
	return nil
}

func StopForTheMove(machine Machine, progress io.Writer, keepWhatWasWritten func() error) error {
	linux, errorValue := thisLinuxBox("stops for a move")
	if errorValue != nil {
		return errorValue
	}
	return linux.stopForTheMove(machine, progress, keepWhatWasWritten)
}

func (linux linuxPlatform) stopForTheMove(machine Machine, progress io.Writer, keepWhatWasWritten func() error) error {
	writers, followers := linux.writersAndFollowers()
	behind, messenger := apartFromTheMessenger(writers)
	if errorValue := disableNow(machine, behind, progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be stopped for the move: %w", errorValue)
	}
	if errorValue := disableNow(machine, messenger, progress); errorValue != nil {
		return fmt.Errorf("the company's messenger could not be stopped for the move: %w", errorValue)
	}
	if errorValue := keepWhatWasWritten(); errorValue != nil {
		return errorValue
	}
	if errorValue := disableNow(machine, linux.dataServiceUnits(), progress); errorValue != nil {
		return fmt.Errorf("the company's database could not be stopped for the move, so it could still take commits the box moved to never sees: %w", errorValue)
	}
	if errorValue := disableNow(machine, followers, progress); errorValue != nil {
		return fmt.Errorf("the followers that keep the company's logs in the store could not be stopped for the move: %w", errorValue)
	}
	return nil
}

func (platform linuxPlatform) writersAndFollowers() ([]string, []string) {
	followers := []string{blueclaw.CompanyHostWALFollowServiceName + ".service", blueclaw.CompanyHostFileFollowServiceName + ".service"}
	writers := []string{}
	for _, unit := range platform.companyServiceUnits() {
		if !slices.Contains(followers, unit) {
			writers = append(writers, unit)
		}
	}
	return writers, followers
}

var messengerUnits = []string{
	blueclaw.RelayServiceName + ".service",
	blueclaw.BuzzRelayServiceName + ".service",
	blueclaw.BuzzMediaServiceName + ".service",
	blueclaw.BuzzMediaGateServiceName + ".service",
}

func apartFromTheMessenger(units []string) ([]string, []string) {
	behind, messenger := []string{}, []string{}
	for _, unit := range units {
		if slices.Contains(messengerUnits, unit) {
			messenger = append(messenger, unit)
		} else {
			behind = append(behind, unit)
		}
	}
	return behind, messenger
}

func (platform linuxPlatform) dataServiceUnits() []string {
	units := []string{}
	for _, unit := range blueclaw.CompanyHostDataServiceUnits(platform.Layout()) {
		units = append(units, unit.FileName())
	}
	return units
}

func disableNow(machine Machine, units []string, progress io.Writer) error {
	if len(units) == 0 {
		return nil
	}
	return machine.Run("systemctl", append([]string{"disable", "--now"}, units...), nil, progress)
}

func thisLinuxBox(doing string) (linuxPlatform, error) {
	platform, errorValue := ThisMachine()
	if errorValue != nil {
		return linuxPlatform{}, errorValue
	}
	linux, isLinux := platform.(linuxPlatform)
	if !isLinux {
		return linuxPlatform{}, fmt.Errorf("a box %s on a Linux host; on %s the company's services are Homebrew's", doing, platform.Describe())
	}
	return linux, nil
}

func forgetTheCurrentCompany() error {
	errorValue := os.Remove(blueclaw.CompanyHostCurrentPath)
	if errors.Is(errorValue, fs.ErrNotExist) {
		return nil
	}
	return errorValue
}

func HoldsACompany() (bool, error) {
	companyID, errorValue := installedCompanyID()
	return companyID != "", errorValue
}

func StandUp(machine Machine, progress io.Writer) error {
	linux, errorValue := thisLinuxBox("stands up")
	if errorValue != nil {
		return errorValue
	}
	units := append(linux.dataServiceUnits(), linux.companyServiceUnits()...)
	if errorValue := machine.Run("systemctl", append([]string{"enable", "--now"}, units...), nil, progress); errorValue != nil {
		return fmt.Errorf("the company's services could not be started again on this box: %w", errorValue)
	}
	return nil
}

func (linuxPlatform) StartTheBox(machine Machine, progress io.Writer) error {
	return KeepTheBoxSessionFresh(machine, progress)
}
