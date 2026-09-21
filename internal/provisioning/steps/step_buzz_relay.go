package setup

import (
	"errors"
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

var StepBuzzRelay = Step{
	Name: "buzz-relay",
	Deps: []string{"buzz-relay-key"},
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
		if context.Callbacks.InstallBuzzRelayBinariesSSH == nil || context.Callbacks.GetBuzzRelayOwnerPubkey == nil {
			return errors.New("buzz relay callbacks missing")
		}
		ownerPubkey, errorValue := context.Callbacks.GetBuzzRelayOwnerPubkey()
		if errorValue != nil {
			return errorValue
		}
		if errorValue := context.Callbacks.InstallBuzzRelayBinariesSSH(context); errorValue != nil {
			return errorValue
		}

		connection := context.SSH
		if errorValue := startRelayCache(connection); errorValue != nil {
			return errorValue
		}
		if errorValue := startRelayDatabase(connection); errorValue != nil {
			return errorValue
		}
		connection.Run(buzzDatabaseProvisionCommand(ownerPubkey))
		connection.Run(buzzRelayUnitInstallCommand(blueclaw.RelayPublicURL(context.RelayDomain)))

		fmt.Println("  " + context.T("Buzz 릴레이 설치 완료", "Buzz relay installed"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func startRelayCache(connection BoardConnection) error {
	output := connection.Run(withPackageWorkSettled(relayCacheInstallCommand()))
	state := strings.TrimSpace(connection.Run("systemctl is-active redis-server"))
	if state == "active" {
		return nil
	}
	return fmt.Errorf("redis-server is %s after installing it, so the buzz relay that queues through it cannot start: %s",
		state, strings.TrimSpace(output))
}

func relayCacheInstallCommand() string {
	return `if systemctl is-active --quiet redis-server; then exit 0; fi
export DEBIAN_FRONTEND=noninteractive
apt-get install -y -qq ` + blueclaw.BuzzRelayCachePackages() + ` 2>&1 | tail -5
systemctl enable --now redis-server 2>&1 | tail -5`
}

func startRelayDatabase(connection BoardConnection) error {
	output := connection.Run(withPackageWorkSettled(relayDatabaseInstallCommand()))
	state := strings.TrimSpace(connection.Run("systemctl is-active postgresql"))
	if state == "active" {
		return nil
	}
	return fmt.Errorf("postgresql is %s after installing %s, so the buzz relay bound to it cannot start: %s",
		state, blueclaw.BuzzRelayDatabasePackages(), strings.TrimSpace(output))
}

func relayDatabaseInstallCommand() string {
	return `if systemctl is-active --quiet postgresql; then exit 0; fi
export DEBIAN_FRONTEND=noninteractive
apt-get install -y -qq ` + blueclaw.BuzzRelayDatabasePackages() + ` 2>&1 | tail -5
systemctl enable --now postgresql 2>&1 | tail -5
sleep 1`
}

func buzzDatabaseProvisionCommand(ownerPubkey string) string {
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
{
  printf 'DATABASE_URL=postgres://` + blueclaw.BuzzRelayDatabaseUser + `:%s@localhost/` + blueclaw.BuzzRelayDatabaseName + `?sslmode=disable\n' "$BUZZ_DB_PASS"
  printf 'RELAY_OWNER_PUBKEY=%s\n' '` + ownerPubkey + `'
} > ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath + `
chmod 600 ` + blueclaw.BuzzRelayDatabaseEnvironmentFilePath
}

func buzzRelayUnitInstallCommand(relayCanonicalURL string) string {
	return `cat > ` + blueclaw.BuzzRelayServicePath + ` <<'BUZZRELAYUNITEOF'
` + blueclaw.BuzzRelayServiceUnit(relayCanonicalURL) + `BUZZRELAYUNITEOF
systemctl daemon-reload
systemctl enable buzz-relay
systemctl restart buzz-relay`
}
