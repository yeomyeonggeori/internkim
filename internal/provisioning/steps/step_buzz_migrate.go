package setup

import (
	"errors"
	"fmt"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBuzzMigrate = Step{
	Name: "buzz-migrate",
	Deps: []string{"buzz-relay", "buzz-media"},
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

		fmt.Println(context.SSH.Run(buzzMigrateInspectCommand()))
		fmt.Println(context.SSH.Run(buzzMigrateSnapshotCommand()))
		fmt.Println(context.SSH.Run(buzzMigrateWipeCommand()))
		fmt.Println(context.SSH.Run(buzzMigrateLaunchCommand()))
		fmt.Println("  " + context.T("임포트를 백그라운드로 시작함 — /tmp/buzz-migrate.log 폴링", "import launched in background — poll /tmp/buzz-migrate.log"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func buzzMigrateInspectCommand() string {
	return `MM_TOKEN=$(cat ` + blueclaw.BlueclawMattermostTokenPath + `)
echo "=== Mattermost teams ==="
curl -fsS -H "Authorization: Bearer $MM_TOKEN" ` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/teams | jq -r '.[] | "\(.name)\t\(.display_name)"'`
}

func buzzMigrateSnapshotCommand() string {
	return `if [ ! -f ` + blueclaw.BuzzPremigrateSnapshotPath + ` ]; then
  su - postgres -c "pg_dump ` + blueclaw.BuzzRelayDatabaseName + `" > ` + blueclaw.BuzzPremigrateSnapshotPath + `
  echo "=== pre-migrate snapshot: $(wc -c < ` + blueclaw.BuzzPremigrateSnapshotPath + `) bytes ==="
else
  echo "=== pre-migrate snapshot already exists (preserved) ==="
fi`
}

func buzzMigrateWipeCommand() string {
	return `systemctl stop ` + blueclaw.BuzzRelayServiceName + `
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
  curl -fsS --max-time 3 http://` + blueclaw.BuzzRelayBindAddress + `/_readiness >/dev/null 2>&1 && break
  sleep 1
done
echo "=== buzz DB wiped, relay in import mode (membership off, limits relaxed), ready ==="`
}

func buzzMigrateLaunchCommand() string {
	return `MM_TOKEN=$(cat ` + blueclaw.BlueclawMattermostTokenPath + `)
SEED=$(cat /root/.internkim/secrets/buzz-key-seed)
DB_URL=$(grep '^DATABASE_URL=' ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + ` | head -1 | sed 's/^DATABASE_URL=//')
TEAM=$(curl -fsS -H "Authorization: Bearer $MM_TOKEN" ` + blueclaw.BlueclawMattermostLocalURL + `/api/v4/teams | jq -r '.[0].name')
rm -f ` + blueclaw.BuzzMigrateMarkerPath + `
cat > /tmp/buzz-migrate-run.sh <<RUNEOF
export DATABASE_URL="$DB_URL"
` + blueclaw.BuzzMigrateBinaryPath + ` \
  --mattermost-url ` + blueclaw.BlueclawMattermostLocalURL + ` \
  --mattermost-token "$MM_TOKEN" \
  --team "$TEAM" \
  --buzz-database-url "$DB_URL" \
  --buzz-admin ` + blueclaw.BuzzAdminBinaryPath + ` \
  --key-seed "$SEED" \
  --relay-url ws://` + blueclaw.BuzzRelayBindAddress + ` \
  --relay-http-url http://` + blueclaw.BuzzRelayBindAddress + ` \
  --community-host ` + blueclaw.BuzzRelayBindAddress + ` \
  --orphan-root-title "이전 대화" \
  && touch ` + blueclaw.BuzzMigrateMarkerPath + ` && echo "MIGRATE_DONE_OK" || echo "MIGRATE_DONE_FAIL"
rm -f ` + blueclaw.BuzzRelayImportOverrideEnvPath + `
systemctl restart ` + blueclaw.BuzzRelayServiceName + `
echo "MIGRATE_PRODUCTION_MODE_RESTORED"
RUNEOF
chmod 700 /tmp/buzz-migrate-run.sh
echo "=== importing team: $TEAM (background) ==="
setsid nohup bash /tmp/buzz-migrate-run.sh > /tmp/buzz-migrate.log 2>&1 < /dev/null &
sleep 1
echo "launched"`
}
