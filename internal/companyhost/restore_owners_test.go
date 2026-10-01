package companyhost

import (
	"strings"
	"testing"
)

const deviceSchemaScript = `--
-- PostgreSQL database dump
--
CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;
COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';
CREATE TABLE public.admin_audit_log (
    id bigint NOT NULL,
    note text DEFAULT 'ALTER x OWNER TO nobody;'::text
);
ALTER TABLE public.admin_audit_log OWNER TO blueclaw;
ALTER SEQUENCE public.admin_audit_log_id_seq OWNER TO blueclaw;
ALTER FUNCTION public.touch() OWNER TO "Device Owner";
ALTER TABLE public.message OWNER TO internkim;
ALTER SCHEMA public OWNER TO pg_database_owner;
`

func TestADumpUnderTheDevicesRolesNamesEachOwnerOnce(t *testing.T) {
	roles, errorValue := foreignOwners(strings.NewReader(deviceSchemaScript))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(roles, "|") != `"Device Owner"|blueclaw` {
		t.Fatalf("foreign owners = %q", roles)
	}
}

func TestAHostsOwnDumpNamesNoForeignOwner(t *testing.T) {
	roles, errorValue := foreignOwners(strings.NewReader("ALTER TABLE public.task OWNER TO internkim;\n"))
	if errorValue != nil || len(roles) != 0 {
		t.Fatalf("a dump owned by %s needs no roles made, got %q, %v", databaseRoleName, roles, errorValue)
	}
}

func TestForeignOwnersAreMadeForTheRestoreAndGivenAway(t *testing.T) {
	roles := []string{"blueclaw"}
	if creation := foreignOwnerCreation(roles); creation != "CREATE ROLE blueclaw NOLOGIN;\n" {
		t.Fatalf("creation = %q", creation)
	}
	release := foreignOwnerRelease("blueclaw", roles)
	for _, statement := range []string{`\connect blueclaw`, "REASSIGN OWNED BY blueclaw TO internkim;", "DROP OWNED BY blueclaw;", `\connect postgres`, "DROP ROLE blueclaw;"} {
		if !strings.Contains(release, statement+"\n") {
			t.Errorf("the release lacks %q:\n%s", statement, release)
		}
	}
	if strings.Index(release, "REASSIGN") > strings.Index(release, "DROP ROLE") {
		t.Error("a role must give away what it owns before it is dropped")
	}
}

func TestTheMessengerCommunityMovesToThisHostsRelayOnlyWhenItIsTheOnlyOne(t *testing.T) {
	statements := messengerCommunityRehoming()
	for _, clause := range []string{
		`\connect buzz`,
		"to_regclass('public.communities') IS NULL",
		"SET host = '127.0.0.1:3000'",
		"(SELECT count(*) FROM communities) = 1",
		"NOT EXISTS (SELECT 1 FROM communities WHERE lower(host) = lower('127.0.0.1:3000'))",
	} {
		if !strings.Contains(statements, clause) {
			t.Errorf("the re-homing lacks %q:\n%s", clause, statements)
		}
	}
}
