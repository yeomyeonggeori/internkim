package companyhost

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

// The compose file got its databases from an image that created them on first
// boot. A distribution PostgreSQL has a cluster and nothing in it, so the role
// and the two databases are made here, once, idempotently.
//
// Neither schema is created here. Blueclaw applies its own migrations as it
// starts and the messenger applies its own under BUZZ_AUTO_MIGRATE, and both
// create the extensions they need — citext, pg_trgm and pgcrypto are trusted
// extensions a database owner may create. pgvector's control file does not mark
// it trusted, so `vector` is created here, by the cluster's superuser, in the
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
	return createTheVectorExtension(platform, machine, progress)
}

const vectorExtensionName = "vector"

var vectorExtensionQuestion = strings.Join([]string{
	`\pset tuples_only on`,
	`\pset format unaligned`,
	`SELECT current_setting('server_version_num')::int / 10000,`,
	`  EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = '` + vectorExtensionName + `');`,
	``,
}, "\n")

var vectorExtensionCreation = strings.Join([]string{
	`\connect ` + blueclaw.BlueclawDatabaseName,
	`CREATE EXTENSION IF NOT EXISTS ` + vectorExtensionName + `;`,
	``,
}, "\n")

// The memory store's migration skips its embedding tables when the server
// cannot create the extension, and from then on recall matches words alone
// with nothing failing. So a cluster that does not offer it is refused here,
// with the package its own server loads it from.
func createTheVectorExtension(platform companyHostPlatform, machine Machine, progress io.Writer) error {
	var answer bytes.Buffer
	if errorValue := platform.RunDatabaseStatements(machine, vectorExtensionQuestion, &answer); errorValue != nil {
		return fmt.Errorf("PostgreSQL would not say whether it offers the %s extension (%w): %s",
			vectorExtensionName, errorValue, strings.TrimSpace(answer.String()))
	}
	major, isOffered, errorValue := parseVectorExtensionAnswer(answer.String())
	if errorValue != nil {
		return errorValue
	}
	if !isOffered {
		return fmt.Errorf("%s", vectorExtensionRefusal(platform, machine, major))
	}
	if errorValue := platform.RunDatabaseStatements(machine, vectorExtensionCreation, progress); errorValue != nil {
		return fmt.Errorf("PostgreSQL %d offers the %s extension and would not create it in the %s database: %w",
			major, vectorExtensionName, blueclaw.BlueclawDatabaseName, errorValue)
	}
	return nil
}

func parseVectorExtensionAnswer(answer string) (int, bool, error) {
	lines := strings.Split(strings.TrimSpace(answer), "\n")
	fields := strings.Split(strings.TrimSpace(lines[len(lines)-1]), "|")
	if len(fields) != 2 {
		return 0, false, fmt.Errorf("PostgreSQL answered %q when asked for its version and whether it offers the %s extension", answer, vectorExtensionName)
	}
	major, errorValue := strconv.Atoi(fields[0])
	if errorValue != nil {
		return 0, false, fmt.Errorf("PostgreSQL answered %q when asked for its major version", fields[0])
	}
	return major, fields[1] == "t", nil
}

func vectorExtensionRefusal(platform companyHostPlatform, machine Machine, major int) string {
	dependency := blueclaw.VectorExtensionDependencyFor(major)
	missing := []missingPiece{{
		What:            fmt.Sprintf("pgvector for PostgreSQL %d", major),
		Dependency:      dependency,
		HomebrewFormula: dependency.HomebrewFormula,
	}}
	lines := []string{fmt.Sprintf(
		"PostgreSQL %d on this computer has no %s extension, and the agent's memory store finds a memory by what it means only with it; without it every recall would match words alone.",
		major, vectorExtensionName)}
	lines = append(lines, platform.HowToInstallTheseByHand(machine, missing)...)
	return strings.Join(append(lines, "Nothing was removed."), "\n")
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
