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
// extensions a database owner may create. pgvector's control file is not marked
// trusted, so the one extension below is created here, by the superuser, in the
// agent's database; the migration's own CREATE EXTENSION IF NOT EXISTS then
// finds it.

const databasePreparationVariable = "INTERNKIM_COMPANY_HOST_SQL"

func prepareDatabases(platform companyHostPlatform, machine Machine, settings companyHostSettings, progress io.Writer) error {
	if errorValue := platform.StartTheDatabaseAndTheCache(machine); errorValue != nil {
		return errorValue
	}
	statements := databasePreparationStatements(settings.DatabasePassword)
	if errorValue := platform.RunDatabaseStatements(machine, statements, progress); errorValue != nil {
		identity := platform.SupervisorIdentityFor(databaseServiceName)
		explanation := strings.Join(platform.HowToSeeWhyItIsSilent(identity), "\n")
		return fmt.Errorf(
			"PostgreSQL would not create the %s role and the %s and %s databases (%w).\n%s\nNothing was removed",
			databaseRoleName, blueclaw.BlueclawDatabaseName, blueclaw.BuzzRelayDatabaseName, errorValue, explanation)
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
		`\connect ` + blueclaw.BlueclawDatabaseName,
		`CREATE EXTENSION IF NOT EXISTS ` + blueclaw.CompanyHostDatabaseRequiredExtension + `;`,
		``,
	}, "\n")
}
