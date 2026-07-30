package blueclaw

import (
	"strings"
	"testing"
)

func TestBuzzRelayServiceUnitCarriesRequiredContract(t *testing.T) {
	unit := BuzzRelayServiceUnit("")
	for _, expected := range []string{
		"Environment=RELAY_URL=" + BuzzRelayLocalURL,
		"After=network-online.target time-sync.target postgresql.service redis-server.service",
		"BindsTo=postgresql.service",
		"EnvironmentFile=" + BuzzRelayKeyEnvironmentFilePath,
		"EnvironmentFile=" + BuzzRelayDatabaseEnvironmentFilePath,
		"EnvironmentFile=-" + BuzzRelayS3EnvironmentFilePath,
		"EnvironmentFile=-" + BuzzRelayImportOverrideEnvPath,
		"Environment=BUZZ_BIND_ADDR=" + BuzzRelayBindAddress,
		"Environment=BUZZ_HEALTH_PORT=" + BuzzRelayHealthPort,
		"Environment=REDIS_URL=" + BuzzRelayRedisURL,
		"Environment=BUZZ_AUTO_MIGRATE=1",
		"Environment=BUZZ_REQUIRE_RELAY_MEMBERSHIP=true",
		"Environment=BUZZ_GIT_CONFORMANCE_PROBE=false",
		"ExecStart=" + BuzzRelayBinaryPath,
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("buzz relay unit missing %q, got:\n%s", expected, unit)
		}
	}
	membershipIndex := strings.Index(unit, "Environment=BUZZ_REQUIRE_RELAY_MEMBERSHIP=true")
	overrideIndex := strings.Index(unit, "EnvironmentFile=-"+BuzzRelayImportOverrideEnvPath)
	if overrideIndex < membershipIndex {
		t.Fatalf("import override must load after the membership default so it can win, got membership@%d override@%d", membershipIndex, overrideIndex)
	}
}

func TestBuzzRelayServiceUnitUsesPublicRelayURL(t *testing.T) {
	unit := BuzzRelayServiceUnit("wss://example-device-relay.example.test")
	if !strings.Contains(unit, "Environment=RELAY_URL=wss://example-device-relay.example.test") {
		t.Fatalf("relay unit missing public RELAY_URL, got:\n%s", unit)
	}
	if strings.Contains(unit, "Environment=RELAY_URL="+BuzzRelayLocalURL) {
		t.Fatalf("relay unit must not fall back to loopback when a public URL is supplied, got:\n%s", unit)
	}
	if !strings.Contains(unit, "Environment=BUZZ_BIND_ADDR="+BuzzRelayBindAddress) {
		t.Fatalf("relay unit must keep binding loopback, got:\n%s", unit)
	}
}

func TestChatdServiceUnitRelayURLAndCACerts(t *testing.T) {
	loopback := ChatdServiceUnit("")
	if !strings.Contains(loopback, "Environment=CHATD_BUZZ_RELAY_URL=ws://"+BuzzRelayBindAddress) {
		t.Fatalf("chatd unit missing loopback relay URL fallback, got:\n%s", loopback)
	}
	if !strings.Contains(loopback, "Environment=NODE_EXTRA_CA_CERTS="+BuzzRelayCertificatePath) {
		t.Fatalf("chatd unit missing relay CA cert, got:\n%s", loopback)
	}
	public := ChatdServiceUnit("wss://example-device-relay.example.test")
	if !strings.Contains(public, "Environment=CHATD_BUZZ_RELAY_URL=wss://example-device-relay.example.test") {
		t.Fatalf("chatd unit missing public relay URL, got:\n%s", public)
	}
}

func TestMinioServiceUnitServesMediaBucketBackend(t *testing.T) {
	unit := MinioServiceUnit()
	for _, expected := range []string{
		"EnvironmentFile=" + MinioEnvironmentFilePath,
		MinioBinaryPath + " server " + MinioDataPath,
		"--address " + MinioAddress,
	} {
		if !strings.Contains(unit, expected) {
			t.Fatalf("minio unit missing %q, got:\n%s", expected, unit)
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
