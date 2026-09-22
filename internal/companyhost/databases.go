package companyhost

import (
	"fmt"
	"io"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

// The compose file got its databases from an image that created them on first
// boot. A distribution PostgreSQL has a cluster and nothing in it, so the role
// and the two databases are made here, once, idempotently.
//
// Neither schema is created here. Blueclaw applies its own migrations as it
// starts and the messenger applies its own under BUZZ_AUTO_MIGRATE, and both
// create the extensions they need — citext, pg_trgm and pgcrypto are trusted
// extensions a database owner may create, so nothing below needs to.

const databasePreparationVariable = "INTERNKIM_COMPANY_HOST_SQL"

func prepareDatabases(machine Machine, settings companyHostSettings, progress io.Writer) error {
	if errorValue := startDistributionServices(machine); errorValue != nil {
		return errorValue
	}
	statements := databasePreparationStatements(settings.DatabasePassword)
	// The password reaches psql through the environment and never through a
	// command line, which every account on this box can read out of /proc.
	arguments := []string{
		"-u", "postgres", "--",
		"sh", "-c", `printf '%s' "$` + databasePreparationVariable + `" | psql --set ON_ERROR_STOP=1 --quiet --no-psqlrc --dbname postgres`,
	}
	environment := []string{databasePreparationVariable + "=" + statements}
	if errorValue := machine.Run("runuser", arguments, environment, progress); errorValue != nil {
		return fmt.Errorf(
			"PostgreSQL would not create the %s role and the %s and %s databases (%w).\n"+
				"`systemctl status postgresql` says whether the server is running, and\n"+
				"`journalctl -u postgresql -n 30` says why it is not. Nothing was removed",
			databaseRoleName, blueclaw.BlueclawDatabaseName, blueclaw.BuzzRelayDatabaseName, errorValue)
	}
	return nil
}

// databasePreparationStatements is a pure function of the password so a test can
// read what would run without a PostgreSQL to run it against. It is written to
// be run twice: a company host is installed again whenever its connection file
// is downloaded again.
func databasePreparationStatements(password string) string {
	quotedPassword := "'" + strings.ReplaceAll(password, "'", "''") + "'"
	return strings.Join([]string{
		`DO $$ BEGIN`,
		`  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '` + databaseRoleName + `') THEN`,
		`    CREATE ROLE ` + databaseRoleName + ` LOGIN PASSWORD ` + quotedPassword + `;`,
		`  ELSE`,
		`    ALTER ROLE ` + databaseRoleName + ` WITH LOGIN PASSWORD ` + quotedPassword + `;`,
		`  END IF;`,
		`END $$;`,
		`SELECT 'CREATE DATABASE ` + blueclaw.BlueclawDatabaseName + ` OWNER ` + databaseRoleName + `'`,
		`  WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = '` + blueclaw.BlueclawDatabaseName + `')\gexec`,
		`SELECT 'CREATE DATABASE ` + blueclaw.BuzzRelayDatabaseName + ` OWNER ` + databaseRoleName + `'`,
		`  WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = '` + blueclaw.BuzzRelayDatabaseName + `')\gexec`,
		``,
	}, "\n")
}
