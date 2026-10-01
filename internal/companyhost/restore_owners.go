package companyhost

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const ownerClausePrefix = "OWNER TO "

// pg_dump writes every owner as `ALTER <object> OWNER TO <role>;`, quoting the
// role as an identifier when it needs it. A name starting with pg_ is one of
// PostgreSQL's own roles and is never one to make or drop.
func foreignOwners(script io.Reader) ([]string, error) {
	found := map[string]bool{}
	scanner := bufio.NewScanner(script)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "ALTER ") || !strings.HasSuffix(line, ";") {
			continue
		}
		index := strings.LastIndex(line, " "+ownerClausePrefix)
		if index < 0 {
			continue
		}
		role := strings.TrimSuffix(line[index+len(ownerClausePrefix)+1:], ";")
		if role == databaseRoleName || strings.HasPrefix(strings.Trim(role, `"`), "pg_") {
			continue
		}
		found[role] = true
	}
	if errorValue := scanner.Err(); errorValue != nil {
		return nil, errorValue
	}
	roles := make([]string, 0, len(found))
	for role := range found {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles, nil
}

// CREATE ROLE fails on a role the cluster already has, so a dump naming one of
// this cluster's own roles is refused before anything it owns is given away.
func foreignOwnerCreation(roles []string) string {
	statements := []string{}
	for _, role := range roles {
		statements = append(statements, "CREATE ROLE "+role+" NOLOGIN;")
	}
	return strings.Join(append(statements, ""), "\n")
}

func foreignOwnerRelease(database string, roles []string) string {
	statements := []string{`\connect ` + database}
	for _, role := range roles {
		statements = append(statements, "REASSIGN OWNED BY "+role+" TO "+databaseRoleName+";", "DROP OWNED BY "+role+";")
	}
	statements = append(statements, `\connect postgres`)
	for _, role := range roles {
		statements = append(statements, "DROP ROLE "+role+";")
	}
	return strings.Join(append(statements, ""), "\n")
}

func restoreUnderForeignOwners(platform backupPlatform, archive hostbackup.Archive, machine Machine, database string, member hostbackup.Member, progress io.Writer) error {
	var roles []string
	if errorValue := archive.ReadMember(member.Name, func(input io.Reader) error {
		owners, errorValue := platform.OwnersInDump(machine, input)
		roles = owners
		return errorValue
	}); errorValue != nil {
		return errorValue
	}
	if len(roles) > 0 {
		fmt.Fprintf(progress, "  %s was dumped under %s; the restore gives what they own to %s\n", database, strings.Join(roles, ", "), databaseRoleName)
		if errorValue := platform.RunDatabaseStatements(machine, foreignOwnerCreation(roles), progress); errorValue != nil {
			return fmt.Errorf("the roles the %s dump names could not be made for the restore: %w", database, errorValue)
		}
	}
	if errorValue := archive.ReadMember(member.Name, func(input io.Reader) error {
		return platform.RestoreDatabase(machine, database, input)
	}); errorValue != nil {
		return errorValue
	}
	if len(roles) == 0 {
		return nil
	}
	if errorValue := platform.RunDatabaseStatements(machine, foreignOwnerRelease(database, roles), progress); errorValue != nil {
		return fmt.Errorf("the %s database was restored, but what %s own could not be given to %s: %w", database, strings.Join(roles, ", "), databaseRoleName, errorValue)
	}
	return nil
}
