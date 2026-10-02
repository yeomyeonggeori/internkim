package companyhost

import (
	"bytes"
	"fmt"
	"io"
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
			"backup and restore run on a Linux host, where the package runs the company's own database. On %s the database is Homebrew's, and this command does not manage it",
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
	units := []string{blueclaw.BoxServiceName + ".service"}
	for _, unit := range blueclaw.CompanyHostSystemdUnits(platform.Layout()) {
		if !unit.IsDataService() {
			units = append(units, unit.FileName())
		}
	}
	if errorValue := machine.Run("systemctl", append([]string{"stop"}, units...), nil, progress); errorValue != nil {
		return fmt.Errorf("the host's services could not be stopped, so nothing was restored: %w", errorValue)
	}
	return nil
}

func (linuxPlatform) StartTheBox(machine Machine, progress io.Writer) error {
	return KeepTheBoxSessionFresh(machine, progress)
}
