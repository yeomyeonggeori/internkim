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

// The jailer stages the kernel and both images into its chroot by hard link, and the
// bind mounts ProtectSystem= introduces put source and destination on different
// devices, so confinement and Firecracker cannot both hold. Cloud Hypervisor opens
// the same paths where they already are.
func blueclawFilesystemConfinement() string {
	if BlueclawVirtualMachineMonitor != CloudHypervisorMonitorName {
		return ""
	}
	return `NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=false
PrivateTmp=true
ReadWritePaths=` + strings.Join(blueclawWritablePaths(), " ") + `
`
}

// Cloud Hypervisor opens the guest rootfs where it is installed, and the guest boots
// it writable. The jailer reached the same inode through a hard link, so the image was
// always mutated in place; only the path that opens it is new.
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

func GraphitiMemorydServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Blueclaw Graphiti Memory Daemon
After=network-online.target time-sync.target internkim-capabilityd.service
Wants=network-online.target time-sync.target internkim-capabilityd.service

[Service]
User=%s
Environment=HOME=%s
Environment=BLUECLAW_GRAPHITI_KUZU_PATH=%s
Environment=BLUECLAW_GRAPHITI_LISTEN_ADDRESS=127.0.0.1
Environment=BLUECLAW_GRAPHITI_PORT=7791
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, BlueclawUser, BlueclawHomePath, GraphitiKuzuPath, GraphitiMemorydPath)
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
ExecStart=%s --vsock-port %d --companion-url http://127.0.0.1:18080/_internkim/companion
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, capabilitydStartCommand(localInferenceMode), CapabilityVSockPort)
}

func capabilitydStartCommand(localInferenceMode string) string {
	command := fmt.Sprintf("%s --mattermost-url %s --mattermost-token %s", CapabilitydBinaryPath, BlueclawMattermostLocalURL, BlueclawMattermostTokenPath)
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
ExecStart=%s -buzz-relay-url %s -buzz-database-url-path %s -buzz-admin-command %s -buzz-key-seed-path %s -buzz-relay-key-path %s -buzz-account-links %s -chatd-endpoint %s -chatd-platform buzz
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, AdmindBinaryPath, BuzzRelayLocalURL, BuzzRelayDatabaseEnvironmentFilePath, BuzzAdminBinaryPath, "/root/.internkim/secrets/buzz-key-seed", BuzzRelayKeyEnvironmentFilePath, BuzzAccountLinksPath, ChatdEndpoint)
}

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
	return "curl --max-time 5 -fsS http://" + BuzzRelayBindAddress + "/_readiness >/dev/null && echo ok || echo no"
}

func ChatdServiceUnit(relayPublicURL string) string {
	relayURL := relayPublicURL
	if relayURL == "" {
		relayURL = "ws://" + BuzzRelayBindAddress
	}
	return fmt.Sprintf(`[Unit]
Description=Buzz chatd bridge
After=network-online.target buzz-relay.service internkim-admind.service
Wants=network-online.target

[Service]
User=root
EnvironmentFile=%s
Environment=CHATD_BOT_USER_NAME=%s
Environment=CHATD_BUZZ_RELAY_URL=%s
Environment=NODE_EXTRA_CA_CERTS=%s
Environment=NODE_TLS_REJECT_UNAUTHORIZED=0
Environment=CHATD_BUZZ_ACCOUNT_LINKS_PATH=%s
Environment=CHATD_LISTEN_PORT=%s
Environment=CHATD_ADMIND_BASE_URL=%s
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, ChatdEnvironmentFilePath, ChatdBotUserName, relayURL, BuzzRelayCertificatePath, BuzzAccountLinksPath, ChatdListenPort, AdmindBaseURL, ChatdBinaryPath)
}

func ChatdHealthCheckCommand() string {
	return "curl --max-time 5 -fsS " + ChatdEndpoint + "/health >/dev/null && echo ok || echo no"
}

func MinioServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz MinIO Object Store
After=network-online.target
Wants=network-online.target

[Service]
User=root
EnvironmentFile=%s
ExecStart=%s server %s --address %s --console-address %s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, MinioEnvironmentFilePath, MinioBinaryPath, MinioDataPath, MinioAddress, MinioConsoleAddress)
}

func MinioHealthCheckCommand() string {
	return "curl --max-time 5 -fsS http://" + MinioAddress + "/minio/health/ready >/dev/null && echo ok || echo no"
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
