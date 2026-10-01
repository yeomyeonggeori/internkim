package blueclaw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
)

func BlueclawServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Blueclaw
After=network-online.target time-sync.target internkim-capabilityd.service
Wants=network-online.target time-sync.target internkim-capabilityd.service

[Service]
User=root
Environment=RUST_LOG=%s
ExecStart=%s -runtime %s
Restart=on-failure
RestartSec=2
KillMode=mixed
TimeoutStopSec=30
%s
[Install]
WantedBy=multi-user.target
`, BlueclawRuntimeLogLevel, BlueclawSupervisorBinaryPath, BlueclawRuntimeConfigPath, blueclawFilesystemConfinement())
}

// Cloud Hypervisor opens the kernel and the images where they are installed, so the
// unit can confine the filesystem around it.
//
// The instance directory is the supervisor's to fill, but ProtectSystem=strict
// refuses to start a unit whose ReadWritePaths names a directory that is not
// there yet, so systemd is asked to make it.
func blueclawFilesystemConfinement() string {
	if BlueclawVirtualMachineMonitor != CloudHypervisorMonitorName {
		return ""
	}
	return `NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=false
PrivateTmp=true
StateDirectory=` + filepath.Base(BlueclawRuntimeInstanceDirectoryPath) + `
ReadWritePaths=` + strings.Join(blueclawWritablePaths(), " ") + `
`
}

// Cloud Hypervisor opens the guest rootfs where it is installed, and the guest boots
// it writable, so the install path must stay writable.
func blueclawWritablePaths() []string {
	return []string{
		BlueclawRuntimeInstallPath,
		BlueclawRuntimeInstanceDirectoryPath,
		filepath.Dir(BlueclawWorkspaceImagePath),
		BlueclawSupervisorLogDirectoryPath,
		BlueclawWorkspacePath,
		"/run",
	}
}

func CapabilitydServiceUnit() string {
	return CapabilitydServiceUnitForLocalInferenceMode("")
}

func CapabilitydServiceUnitForLocalInferenceMode(localInferenceMode string) string {
	return fmt.Sprintf(`[Unit]
Description=internkim Capability Daemon
After=network-online.target time-sync.target mattermost.service internkim-admind.service
Wants=network-online.target time-sync.target internkim-admind.service

[Service]
User=root
RuntimeDirectory=internkim
RuntimeDirectoryPreserve=yes
ExecStart=%s --vsock-port %d --companion-url http://127.0.0.1:18080/_internkim/companion
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, capabilitydStartCommand(localInferenceMode), CapabilityVSockPort)
}

func capabilitydStartCommand(localInferenceMode string) string {
	command := fmt.Sprintf("%s --chatd-endpoint %s --chatd-platform buzz", CapabilitydBinaryPath, ChatdEndpoint)
	if modelName := strings.TrimSpace(os.Getenv(BlueclawTestModelEnvironment)); modelName != "" {
		command += " --openrouter-model " + modelName
	}
	if localInferenceMode != "" {
		command += " --local-inference-mode " + localInferenceMode
	}
	if LocalOnlyEnabled() {
		command += " --local-only"
	}
	return command
}

func AdmindServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim Admin Gateway
After=network-online.target time-sync.target mattermost.service
Wants=network-online.target time-sync.target

[Service]
User=root
RuntimeDirectory=internkim
RuntimeDirectoryPreserve=yes
ExecStart=%s -buzz-relay-url %s -buzz-database-url-path %s -buzz-admin-command %s -buzz-key-seed-path %s -buzz-relay-key-path %s -buzz-account-links %s -chatd-endpoint %s -chatd-platform buzz
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, AdmindBinaryPath, BuzzRelayLocalURL, BuzzRelayDatabaseEnvironmentFilePath, BuzzAdminBinaryPath, "/root/.internkim/secrets/buzz-key-seed", BuzzRelayKeyEnvironmentFilePath, BuzzAccountLinksPath, ChatdEndpoint)
}

// The relay verifies a NIP-42 auth event against the url it believes it answers
// at, so naming a scheme it does not serve locks every client out. Without a
// public host it terminates no TLS, and loopback is ws.
func BuzzRelayServiceUnit(relayPublicURL string) string {
	relayURL := relayPublicURL
	if relayURL == "" {
		relayURL = BuzzRelayLocalURL
	}
	return fmt.Sprintf(`[Unit]
Description=Buzz Relay
After=network-online.target time-sync.target postgresql.service redis-server.service
Wants=network-online.target time-sync.target redis-server.service
BindsTo=postgresql.service

[Service]
User=root
EnvironmentFile=%s
EnvironmentFile=%s
EnvironmentFile=-%s
Environment=BUZZ_BIND_ADDR=%s
Environment=BUZZ_HEALTH_PORT=%s
Environment=REDIS_URL=%s
Environment=RELAY_URL=%s
Environment=BUZZ_AUTO_MIGRATE=1
Environment=BUZZ_REQUIRE_RELAY_MEMBERSHIP=true
Environment=BUZZ_GIT_CONFORMANCE_PROBE=false
Environment=BUZZ_RATE_LIMIT_HUMAN_MESSAGES_PER_MIN=1200
Environment=BUZZ_RATE_LIMIT_HUMAN_WS_EVENTS_PER_SEC=100
EnvironmentFile=-%s
ExecStart=%s
Restart=on-failure
RestartSec=2
KillMode=mixed
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
`, BuzzRelayKeyEnvironmentFilePath, BuzzRelayDatabaseEnvironmentFilePath, BuzzRelayS3EnvironmentFilePath, BuzzRelayBindAddress, BuzzRelayHealthPort, BuzzRelayRedisURL, relayURL, BuzzRelayImportOverrideEnvPath, BuzzRelayBinaryPath)
}

func BuzzRelayHealthCheckCommand() string {
	return "curl --max-time 5 -fsS " + BuzzRelayReadinessURL() + " >/dev/null && echo ok || echo no"
}

// BuzzRelayReadinessURL is the one address anything asking "is the messenger
// ready" asks, on the device and on the packaged host alike.
func BuzzRelayReadinessURL() string {
	return "http://" + BuzzRelayBindAddress + BuzzRelayReadinessPath
}

func ChatdLegacyTLSDropInPaths() []string {
	return []string{ChatdDropInDirectory + "/tls.conf", ChatdDropInDirectory + "/tls-debug.conf"}
}

func ChatdServiceUnit(relayPublicURL string) string {
	relayURL := relayPublicURL
	if relayURL == "" {
		relayURL = "ws://" + BuzzRelayBindAddress
	}
	return fmt.Sprintf(`[Unit]
Description=Buzz chatd bridge
After=network-online.target buzz-relay.service internkim-admind.service blueclaw.service
Wants=network-online.target

[Service]
User=root
EnvironmentFile=%s
Environment=CHATD_BOT_USER_NAME=%s
Environment=CHATD_BUZZ_RELAY_URL=%s
Environment=`+BuzzRelayCertificateVariable+`=%s
Environment=CHATD_BUZZ_ACCOUNT_LINKS_PATH=%s
Environment=CHATD_LISTEN_HOSTNAME=`+ChatdListenHostname+`
Environment=CHATD_LISTEN_PORT=%s
Environment=CHATD_ADMIND_BASE_URL=%s
Environment=CHATD_STATE_DIRECTORY=%s
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, ChatdEnvironmentFilePath, ChatdBotUserName, relayURL, BuzzRelayCertificatePath, BuzzAccountLinksPath, ChatdListenPort, AdmindBaseURL, ChatdStateDirectoryPath, ChatdBinaryPath)
}

func ChatdHealthCheckCommand() string {
	return "curl --max-time 5 -fsS " + ChatdEndpoint + ChatdHealthPath + " >/dev/null && echo ok || echo no"
}

// BuzzMediaServiceUnit starts the S3 gateway the relay stores attachments
// through. The posix backend serves the bucket directory as-is, so no client
// creates it and nothing about the bucket lives inside a storage format.
//
// --versioning-dir is deliberately absent, and
// TestBuzzMediaUnitDoesNotEnableVersioning is why: without it the gateway
// refuses PutBucketVersioning outright, which makes the one incompatibility
// measured against versitygw 1.8.0 unreachable. Read that test before adding
// the flag.
func BuzzMediaServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz Media Store
After=network-online.target
Wants=network-online.target

[Service]
User=root
EnvironmentFile=%s
ExecStart=%s --port %s --health %s posix %s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, BuzzMediaEnvironmentFilePath, BuzzMediaBinaryPath, BuzzMediaAddress, BuzzMediaHealthPath, BuzzMediaRootPath)
}

func BuzzMediaHealthCheckCommand() string {
	return "curl --max-time 5 -fsS http://" + BuzzMediaAddress + BuzzMediaHealthPath + " >/dev/null && echo ok || echo no"
}

func LlamaCppServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim llama.cpp Server
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=root
Environment=LD_LIBRARY_PATH=%s
ExecStart=%s -m %s --model-draft %s --spec-type draft-mtp --spec-draft-n-max 4 --host %s --port %s -ngl 99 --spec-draft-ngl 99 -fa on --chat-template gemma --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=180

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppModelPath, locallm.LlamaCppDraftModelPath, locallm.LlamaCppHost, locallm.LlamaCppPort)
}

func LlamaCppEmbeddingServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim llama.cpp Embedding Server
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=root
Environment=LD_LIBRARY_PATH=%s
ExecStart=%s -m %s --host %s --port %s -ngl 0 --embeddings --pooling cls --batch-size %s --ubatch-size %s --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppEmbeddingModelPath, locallm.LlamaCppHost, locallm.LlamaCppEmbeddingPort, locallm.LlamaCppEmbeddingBatchSize, locallm.LlamaCppEmbeddingUBatchSize)
}
