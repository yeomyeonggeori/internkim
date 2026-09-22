package setup

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const (
	buzzMigrateTeamMarker     = "BUZZ_MIGRATE_TEAM="
	buzzMigrateSnapshotMarker = "BUZZ_MIGRATE_SNAPSHOT_OK"
	buzzMigrateWipeMarker     = "BUZZ_MIGRATE_WIPE_OK"
)

var StepBuzzMigrate = Step{
	Name: "buzz-migrate",
	Deps: []string{"buzz-relay", "buzz-media", "buzz-public-host"},
	Title: func(context *Context) string {
		return context.T("Mattermost 히스토리 마이그레이션 중...", "Migrating Mattermost history...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		return trimmedRun(context, "test -f "+blueclaw.BuzzMigrateMarkerPath+" && echo yes || echo no") == "yes"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		if context.Callbacks.InstallBuzzRelayBinariesSSH == nil {
			return errors.New("buzz relay binaries callback missing")
		}
		if errorValue := context.Callbacks.InstallBuzzRelayBinariesSSH(context); errorValue != nil {
			return errorValue
		}
		if trimmedRun(context, "test -x "+blueclaw.BuzzMigrateBinaryPath+" && echo yes || echo no") != "yes" {
			fmt.Println("  " + context.T(
				"buzz-migrate 없음 — Mattermost 히스토리 마이그레이션 건너뜀",
				"buzz-migrate absent - skipping Mattermost history migration",
			))
			return nil
		}

		connection := context.SSH
		inspectOutput := connection.Run(buzzMigrateTeamCommand())
		fmt.Println(inspectOutput)
		team := valueAfterMarker(inspectOutput, buzzMigrateTeamMarker)
		if team == "" {
			fmt.Println("  " + context.T(
				"가져올 Mattermost가 없음 — 데이터베이스를 그대로 둠",
				"no Mattermost to import from - leaving the database as it is",
			))
			return nil
		}

		if errorValue := prepareBuzzDatabaseForImport(connection); errorValue != nil {
			return errorValue
		}

		fmt.Println(connection.Run(buzzMigrateLaunchCommand(team)))
		fmt.Println("  " + context.T("임포트를 백그라운드로 시작함 — /tmp/buzz-migrate.log 폴링", "import launched in background — poll /tmp/buzz-migrate.log"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func prepareBuzzDatabaseForImport(connection BoardConnection) error {
	snapshotOutput := connection.Run(buzzMigrateSnapshotCommand())
	fmt.Println(snapshotOutput)
	if !strings.Contains(snapshotOutput, buzzMigrateSnapshotMarker) {
		return fmt.Errorf("no restorable snapshot of %s was taken, so the import must not wipe it: %s",
			blueclaw.BuzzRelayDatabaseName, strings.TrimSpace(snapshotOutput))
	}
	wipeOutput := connection.Run(buzzMigrateWipeCommand())
	fmt.Println(wipeOutput)
	if !strings.Contains(wipeOutput, buzzMigrateWipeMarker) {
		return fmt.Errorf("preparing %s for the import failed and the relay may be down: %s",
			blueclaw.BuzzRelayDatabaseName, strings.TrimSpace(wipeOutput))
	}
	return nil
}

func valueAfterMarker(output string, marker string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, marker) {
			return strings.TrimSpace(strings.TrimPrefix(trimmedLine, marker))
		}
	}
	return ""
}

func buzzMigrateTeamCommand() string {
	return `MM_TOKEN=$(cat ` + blueclaw.BlueclawMattermostTokenPath + ` 2>/dev/null || echo "")
if [ -z "$MM_TOKEN" ]; then
  echo "=== this machine holds no Mattermost token ==="
  exit 0
fi
TEAMS=$(curl -fsS --max-time 10 -H "Authorization: Bearer $MM_TOKEN" ` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/teams 2>/dev/null || echo "")
if [ -z "$TEAMS" ]; then
  echo "=== Mattermost answered nothing at ` + blueclaw.BlueclawMattermostLocalURL + ` ==="
  exit 0
fi
echo "=== Mattermost teams ==="
printf '%s' "$TEAMS" | jq -r '.[] | "\(.name)\t\(.display_name)"'
printf '` + buzzMigrateTeamMarker + `%s\n' "$(printf '%s' "$TEAMS" | jq -r '.[0].name // empty')"`
}

func buzzMigrateSnapshotCommand() string {
	return `set -e
if [ -s ` + blueclaw.BuzzPremigrateSnapshotPath + ` ]; then
  echo "=== pre-migrate snapshot already exists (preserved) ==="
else
  su - postgres -c "pg_dump ` + blueclaw.BuzzRelayDatabaseName + `" > ` + blueclaw.BuzzPremigrateSnapshotPath + `.partial
  mv ` + blueclaw.BuzzPremigrateSnapshotPath + `.partial ` + blueclaw.BuzzPremigrateSnapshotPath + `
fi
test -s ` + blueclaw.BuzzPremigrateSnapshotPath + `
echo "` + buzzMigrateSnapshotMarker + ` $(wc -c < ` + blueclaw.BuzzPremigrateSnapshotPath + `) bytes"`
}

func buzzMigrateWipeCommand() string {
	return `set -e
systemctl stop ` + blueclaw.ChatdServiceName + ` 2>/dev/null || true
echo "=== chatd stopped for re-import (prevents mirror interference) ==="
systemctl stop ` + blueclaw.BuzzRelayServiceName + `
{
  printf 'BUZZ_REQUIRE_RELAY_MEMBERSHIP=false\n'
  printf 'BUZZ_RATE_LIMIT_HUMAN_MESSAGES_PER_MIN=1000000\n'
  printf 'BUZZ_RATE_LIMIT_HUMAN_API_CALLS_PER_MIN=1000000\n'
  printf 'BUZZ_RATE_LIMIT_HUMAN_WS_EVENTS_PER_SEC=100000\n'
  printf 'BUZZ_MEDIA_UPLOADS_PER_MINUTE=1000000\n'
} > ` + blueclaw.BuzzRelayImportOverrideEnvPath + `
chmod 600 ` + blueclaw.BuzzRelayImportOverrideEnvPath + `
su - postgres -c "dropdb --if-exists ` + blueclaw.BuzzRelayDatabaseName + ` && createdb -O ` + blueclaw.BuzzRelayDatabaseUser + ` ` + blueclaw.BuzzRelayDatabaseName + `"
systemctl daemon-reload
systemctl start ` + blueclaw.BuzzRelayServiceName + `
for attempt in $(seq 1 30); do
  curl -fsS --max-time 3 ` + blueclaw.BuzzRelayReadinessURL() + ` >/dev/null 2>&1 && break
  sleep 1
done
curl -fsS --max-time 3 ` + blueclaw.BuzzRelayReadinessURL() + ` >/dev/null
echo "` + buzzMigrateWipeMarker + ` buzz DB wiped, relay in import mode (membership off, limits relaxed), ready"`
}

func buzzMigrateLaunchCommand(team string) string {
	return `set -e
DB_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
PUBLIC_HOST=$(systemctl show ` + blueclaw.BuzzRelayServiceName + ` -p Environment | tr ' ' '\n' | sed -n 's/^RELAY_URL=//p' | head -1 | sed -E 's#^[a-z]+://##; s#/.*$##')
if [ -z "$PUBLIC_HOST" ]; then echo "the relay names no public host, and an import keyed to a guess lands in a community nothing serves"; exit 1; fi
rm -f ` + blueclaw.BuzzMigrateMarkerPath + `
cat > /tmp/buzz-migrate-run.sh <<RUNEOF
export DATABASE_URL="$DB_URL"
restore_snapshot() {
  su - postgres -c "dropdb --if-exists ` + blueclaw.BuzzRelayDatabaseName + ` && createdb -O ` + blueclaw.BuzzRelayDatabaseUser + ` ` + blueclaw.BuzzRelayDatabaseName + `"
  su - postgres -c "psql -q ` + blueclaw.BuzzRelayDatabaseName + `" < ` + blueclaw.BuzzPremigrateSnapshotPath + `
}
if ` + blueclaw.BuzzMigrateBinaryPath + ` \
  --mattermost-url ` + blueclaw.BlueclawMattermostLocalURL + ` \
  --mattermost-token-path ` + blueclaw.BlueclawMattermostTokenPath + ` \
  --team "` + team + `" \
  --buzz-database-url "$DB_URL" \
  --buzz-admin ` + blueclaw.BuzzAdminBinaryPath + ` \
  --key-seed-path ` + buzzKeySeedDevicePath + ` \
  --bridge-url ` + blueclaw.AdmindBaseURL + `/bridge/api \
  --relay-url wss://$PUBLIC_HOST \
  --relay-http-url https://$PUBLIC_HOST \
  --community-host $PUBLIC_HOST \
  --orphan-root-title "이전 대화"; then
  touch ` + blueclaw.BuzzMigrateMarkerPath + `
  echo "MIGRATE_DONE_OK"
else
  echo "MIGRATE_DONE_FAIL restoring the pre-migrate snapshot"
  restore_snapshot && echo "MIGRATE_SNAPSHOT_RESTORED" || echo "MIGRATE_SNAPSHOT_RESTORE_FAILED"
fi
rm -f ` + blueclaw.BuzzRelayImportOverrideEnvPath + `
systemctl restart ` + blueclaw.BuzzRelayServiceName + `
sleep 2
systemctl start ` + blueclaw.ChatdServiceName + `
echo "MIGRATE_PRODUCTION_MODE_RESTORED_AND_CHATD_STARTED"
RUNEOF
chmod 700 /tmp/buzz-migrate-run.sh
echo "=== importing team: ` + team + ` (background) ==="
setsid nohup bash /tmp/buzz-migrate-run.sh > /tmp/buzz-migrate.log 2>&1 < /dev/null &
sleep 1
echo "launched"`
}
