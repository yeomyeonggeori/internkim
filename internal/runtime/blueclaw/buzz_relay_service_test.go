package blueclaw

import (
	"strings"
	"testing"
)

func TestBuzzRelayServiceUnitCarriesRequiredContract(t *testing.T) {
	unit := BuzzRelayServiceUnit()
	for _, expected := range []string{
		"After=network-online.target time-sync.target postgresql.service redis-server.service",
		"BindsTo=postgresql.service",
		"EnvironmentFile=" + BuzzRelayKeyEnvironmentFilePath,
		"EnvironmentFile=" + BuzzRelayDatabaseEnvironmentFilePath,
		"Environment=BUZZ_BIND_ADDR=" + BuzzRelayBindAddress,
		"Environment=REDIS_URL=" + BuzzRelayRedisURL,
		"Environment=BUZZ_AUTO_MIGRATE=1",
		"Environment=BUZZ_REQUIRE_RELAY_MEMBERSHIP=true",
		"ExecStart=" + BuzzRelayBinaryPath,
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("buzz relay unit missing %q, got:\n%s", expected, unit)
		}
	}
}

func TestAdmindServiceUnitWiresBuzzRelay(t *testing.T) {
	unit := AdmindServiceUnit()
	for _, expected := range []string{
		"-buzz-relay-url " + BuzzRelayLocalURL,
		"-buzz-database-url-path " + BuzzRelayDatabaseEnvironmentFilePath,
		"-buzz-admin-command " + BuzzAdminBinaryPath,
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("admind unit missing %q, got:\n%s", expected, unit)
		}
	}
}
