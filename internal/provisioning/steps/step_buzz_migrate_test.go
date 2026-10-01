package setup

import (
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

type migrateConnection struct {
	team         string
	snapshotOK   bool
	wipeOK       bool
	commandsRun  []string
	teamAnswered bool
}

func (connection *migrateConnection) Run(command string) string {
	connection.commandsRun = append(connection.commandsRun, command)
	switch {
	case strings.HasPrefix(command, "test -x "+blueclaw.BuzzMigrateBinaryPath):
		return "yes"
	case command == buzzMigrateTeamCommand():
		connection.teamAnswered = true
		if connection.team == "" {
			return "=== Mattermost answered nothing at " + blueclaw.BlueclawMattermostLocalURL + " ==="
		}
		return "=== Mattermost teams ===\n" + connection.team + "\tThe Team\n" + buzzMigrateTeamMarker + connection.team
	case command == buzzMigrateSnapshotCommand():
		if connection.snapshotOK {
			return buzzMigrateSnapshotMarker + " 210783 bytes"
		}
		return "pg_dump: error: connection to server failed"
	case command == buzzMigrateWipeCommand():
		if connection.wipeOK {
			return buzzMigrateWipeMarker + " buzz DB wiped"
		}
		return "dropdb: error: database is being accessed by other users"
	}
	return ""
}

func (connection *migrateConnection) SCP(localPath, remotePath string) error {
	return nil
}

func migrateContext(connection BoardConnection) *Context {
	return &Context{
		Backend: BackendSSH,
		SSH:     connection,
		Callbacks: Callbacks{
			InstallBuzzRelayBinariesSSH: func(context *Context) error { return nil },
		},
	}
}

func ranCommandContaining(connection *migrateConnection, fragment string) bool {
	for _, command := range connection.commandsRun {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}

func TestBuzzMigrateWipesNothingWithoutAMattermostToImportFrom(t *testing.T) {
	connection := &migrateConnection{}
	if errorValue := StepBuzzMigrate.Run(migrateContext(connection)); errorValue != nil {
		t.Fatalf("a machine with no Mattermost has nothing to migrate and nothing to fail on, got %v", errorValue)
	}
	if !connection.teamAnswered {
		t.Fatal("the step must ask Mattermost before touching the database")
	}
	if ranCommandContaining(connection, "dropdb") {
		t.Fatal("a machine with no Mattermost must keep its buzz database and its relay members")
	}
}

func TestBuzzMigrateRefusesToWipeWithoutARestorableSnapshot(t *testing.T) {
	connection := &migrateConnection{team: "kim", snapshotOK: false}
	errorValue := StepBuzzMigrate.Run(migrateContext(connection))
	if errorValue == nil {
		t.Fatal("a wipe with no snapshot behind it must not be reported as done")
	}
	if !strings.Contains(errorValue.Error(), "snapshot") {
		t.Fatalf("the failure must name what is missing, got %v", errorValue)
	}
	if ranCommandContaining(connection, "dropdb") {
		t.Fatal("the database must survive a failed snapshot")
	}
}

func TestBuzzMigrateFailsWhenThePreparationDidNot(t *testing.T) {
	connection := &migrateConnection{team: "kim", snapshotOK: true, wipeOK: false}
	errorValue := StepBuzzMigrate.Run(migrateContext(connection))
	if errorValue == nil {
		t.Fatal("a wipe that did not finish must not be reported as ready")
	}
	if ranCommandContaining(connection, "setsid nohup") {
		t.Fatal("no import may be launched against a database the wipe left unknown")
	}
}

func TestBuzzMigrateImportsTheTeamMattermostNamed(t *testing.T) {
	connection := &migrateConnection{team: "kim", snapshotOK: true, wipeOK: true}
	if errorValue := StepBuzzMigrate.Run(migrateContext(connection)); errorValue != nil {
		t.Fatalf("a reachable Mattermost with a team must migrate, got %v", errorValue)
	}
	if !ranCommandContaining(connection, `--team "kim"`) {
		t.Fatal("the import must be handed the team the step already resolved")
	}
}

func TestBuzzMigrateRestoresTheSnapshotWhenTheImportFails(t *testing.T) {
	script := buzzMigrateLaunchCommand("kim")
	if !strings.Contains(script, "restore_snapshot") {
		t.Fatal("a failed import must put back the database the wipe took away")
	}
	if !strings.Contains(script, "< "+blueclaw.BuzzPremigrateSnapshotPath) {
		t.Fatalf("the restore must read the snapshot the step took, got %s", script)
	}
}

func TestBuzzMigrateShellCommandsStopAtTheFirstFailure(t *testing.T) {
	for name, command := range map[string]string{
		"snapshot": buzzMigrateSnapshotCommand(),
		"wipe":     buzzMigrateWipeCommand(),
		"launch":   buzzMigrateLaunchCommand("kim"),
	} {
		if !strings.HasPrefix(command, "set -e\n") {
			t.Fatalf("the %s command walks past its own failures without set -e", name)
		}
	}
}
