package setup

import (
	"fmt"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBuzzRelay = Step{
	Name: "buzz-relay",
	Deps: []string{"binaries", "buzz-relay-key", "mattermost"},
	Title: func(context *Context) string {
		return context.T("Buzz 릴레이 설치 중...", "Installing Buzz relay...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		return trimmedRun(context, "systemctl is-active "+blueclaw.BuzzRelayServiceName) == "active" &&
			trimmedRun(context, blueclaw.BuzzRelayHealthCheckCommand()) == "ok"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		connection := context.SSH

		connection.Run("DEBIAN_FRONTEND=noninteractive apt-get install -y -qq redis-server >/dev/null 2>&1; systemctl enable --now redis-server 2>/dev/null")
		connection.Run("systemctl start postgresql 2>/dev/null; sleep 1")
		connection.Run(buzzDatabaseProvisionCommand())
		connection.Run(buzzRelayUnitInstallCommand())

		fmt.Println("  " + context.T("Buzz 릴레이 설치 완료", "Buzz relay installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func buzzDatabaseProvisionCommand() string {
	return `set -e
BUZZ_DB_PASS=$(cat ` + blueclaw.BuzzRelayDatabasePasswordPath + ` 2>/dev/null || echo "")
if [ -z "$BUZZ_DB_PASS" ]; then
  BUZZ_DB_PASS=$(head -c 12 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 16)
  printf '%s' "$BUZZ_DB_PASS" > ` + blueclaw.BuzzRelayDatabasePasswordPath + `
  chmod 600 ` + blueclaw.BuzzRelayDatabasePasswordPath + `
fi
su - postgres -c "psql -c \"SELECT 1 FROM pg_roles WHERE rolname='` + blueclaw.BuzzRelayDatabaseUser + `'\" | grep -q 1 || psql -c \"CREATE USER ` + blueclaw.BuzzRelayDatabaseUser + ` WITH PASSWORD '$BUZZ_DB_PASS'\""
su - postgres -c "psql -c \"ALTER USER ` + blueclaw.BuzzRelayDatabaseUser + ` WITH PASSWORD '$BUZZ_DB_PASS'\""
su - postgres -c "psql -c \"SELECT 1 FROM pg_database WHERE datname='` + blueclaw.BuzzRelayDatabaseName + `'\" | grep -q 1 || psql -c \"CREATE DATABASE ` + blueclaw.BuzzRelayDatabaseName + ` OWNER ` + blueclaw.BuzzRelayDatabaseUser + `\""
printf 'DATABASE_URL=postgres://` + blueclaw.BuzzRelayDatabaseUser + `:%s@localhost/` + blueclaw.BuzzRelayDatabaseName + `?sslmode=disable\n' "$BUZZ_DB_PASS" > ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + `
chmod 600 ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath
}

func buzzRelayUnitInstallCommand() string {
	return `cat > ` + blueclaw.BuzzRelayServicePath + ` <<'BUZZRELAYUNITEOF'
` + blueclaw.BuzzRelayServiceUnit() + `BUZZRELAYUNITEOF
systemctl daemon-reload
systemctl enable buzz-relay
systemctl restart buzz-relay`
}
