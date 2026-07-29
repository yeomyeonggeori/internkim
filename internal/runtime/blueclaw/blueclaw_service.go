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

[Install]
WantedBy=multi-user.target
`, BlueclawRuntimeLogLevel, BlueclawSupervisorBinaryPath, BlueclawRuntimeConfigPath)
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

func LLMDServiceUnit(isLocalLlamaProvisioned bool) string {
	return LLMDServiceUnitForLocalOnly(LocalOnlyEnabled(), isLocalLlamaProvisioned)
}

func LLMDServiceUnitForLocalOnly(isLocalOnly bool, isLocalLlamaProvisioned bool) string {
	localOnlyValue := 0
	openRouterEnvironment := "Environment=OPENROUTER_API_KEY_PATH=" + LLMDRuntimeOpenRouterKeyPath + "\n"
	networkPolicy := ""
	if isLocalOnly {
		localOnlyValue = 1
		openRouterEnvironment = ""
		networkPolicy = "IPAddressDeny=any\nIPAddressAllow=localhost\n"
	}
	llamaEnvironment := ""
	if isLocalLlamaProvisioned {
		llamaEnvironment = fmt.Sprintf(
			"Environment=BLUECLAW_LLMD_LLAMA_BASE_URL=%s\nEnvironment=BLUECLAW_LLMD_LLAMA_MODEL=local/gemma-4-E2B-it-qat-UD-Q4_K_XL\nEnvironment=BLUECLAW_LLMD_LLAMA_STRUCTURED_OUTPUTS_ENABLED=true\n",
			locallm.LlamaCppBaseURL,
		)
	}
	authCredentialPreStart, openRouterCredentialPreStart := llmdCredentialPreStartCommands(isLocalOnly)
	return fmt.Sprintf(`[Unit]
Description=Blueclaw AI SDK Runtime
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
DynamicUser=yes
RuntimeDirectory=blueclaw-llmd
RuntimeDirectoryMode=0700
UMask=0077
Environment=BLUECLAW_LLMD_SOCKET_PATH=%s
%sEnvironment=BLUECLAW_LLMD_LOCAL_ONLY=%d
Environment=BLUECLAW_LLMD_AUTH_KEY_PATH=%s
%s%s%sExecStart=%s
Restart=on-failure
RestartSec=2
TimeoutStopSec=30
NoNewPrivileges=yes
PrivateTmp=yes
PrivateDevices=yes
ProtectSystem=strict
ProtectHome=read-only
ProtectControlGroups=yes
ProtectKernelModules=yes
ProtectKernelTunables=yes
ProtectHostname=yes
RestrictSUIDSGID=yes
LockPersonality=yes
CapabilityBoundingSet=
AmbientCapabilities=
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
%s
[Install]
WantedBy=multi-user.target
`, LLMDSocketPath, llamaEnvironment, localOnlyValue, LLMDRuntimeAuthKeyPath, openRouterEnvironment, authCredentialPreStart, openRouterCredentialPreStart, LLMDBinaryPath, networkPolicy)
}

func llmdCredentialPreStartCommands(isLocalOnly bool) (string, string) {
	authCredentialCommand := fmt.Sprintf(
		"set -eu; install -m 0400 %s %s; chown --reference=%s %s",
		LLMDServiceAuthKeyPath,
		LLMDRuntimeAuthKeyPath,
		LLMDRuntimeDirectoryPath,
		LLMDRuntimeAuthKeyPath,
	)
	if isLocalOnly {
		return llmdPrivilegedPreStart(authCredentialCommand), llmdPrivilegedPreStart("rm -f " + LLMDRuntimeOpenRouterKeyPath)
	}
	openRouterCredentialCommand := fmt.Sprintf(
		"set -eu; if [ -s %s ]; then install -m 0400 %s %s; chown --reference=%s %s; else rm -f %s; fi",
		LLMDServiceOpenRouterKeyPath,
		LLMDServiceOpenRouterKeyPath,
		LLMDRuntimeOpenRouterKeyPath,
		LLMDRuntimeDirectoryPath,
		LLMDRuntimeOpenRouterKeyPath,
		LLMDRuntimeOpenRouterKeyPath,
	)
	return llmdPrivilegedPreStart(authCredentialCommand), llmdPrivilegedPreStart(openRouterCredentialCommand)
}

func llmdPrivilegedPreStart(command string) string {
	return "ExecStartPre=+/bin/sh -c '" + command + "'\n"
}

func LLMDServiceCredentialInstallCommand(isLocalOnly bool) string {
	openRouterCredentialCommand := "if [ -s " + OpenRouterKeyPath + " ]; then\n" +
		"  install -o root -g root -m 600 " + OpenRouterKeyPath + " " + LLMDServiceOpenRouterKeyPath + "\n" +
		"else\n" +
		"  rm -f " + LLMDServiceOpenRouterKeyPath + "\n" +
		"fi"
	if isLocalOnly {
		openRouterCredentialCommand = "rm -f " + LLMDServiceOpenRouterKeyPath
	}
	return fmt.Sprintf(`mkdir -p %[1]s
if [ ! -s %[2]s ]; then
  umask 077
  head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n' > %[2]s
fi
chown root:root %[2]s
chmod 600 %[2]s
install -d -o root -g root -m 700 %[3]s
install -o root -g root -m 600 %[2]s %[4]s
%[5]s`, filepath.Dir(LLMDAuthKeyPath), LLMDAuthKeyPath, LLMDServiceCredentialDirectoryPath, LLMDServiceAuthKeyPath, openRouterCredentialCommand)
}

func CapabilitydServiceUnit() string {
	return CapabilitydServiceUnitForLocalInferenceMode("")
}

func CapabilitydServiceUnitForLocalInferenceMode(localInferenceMode string) string {
	return fmt.Sprintf(`[Unit]
Description=InternKim Capability Daemon
After=network-online.target time-sync.target mattermost.service blueclaw-llmd.service internkim-admind.service
Wants=network-online.target time-sync.target blueclaw-llmd.service internkim-admind.service

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
Description=InternKim Admin Gateway
After=network-online.target time-sync.target mattermost.service
Wants=network-online.target time-sync.target

[Service]
User=root
ExecStart=%s -buzz-relay-url %s -buzz-database-url-path %s -buzz-admin-command %s -buzz-key-seed-path %s -buzz-account-links %s -chatd-endpoint %s -chatd-platform buzz
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, AdmindBinaryPath, BuzzRelayLocalURL, BuzzRelayDatabaseEnvironmentFilePath, BuzzAdminBinaryPath, "/root/.internkim/secrets/buzz-key-seed", BuzzAccountLinksPath, ChatdEndpoint)
}

func BuzzRelayServiceUnit() string {
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
EnvironmentFile=-%s
ExecStart=%s
Restart=on-failure
RestartSec=2
KillMode=mixed
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
`, BuzzRelayKeyEnvironmentFilePath, BuzzRelayDatabaseEnvironmentFilePath, BuzzRelayS3EnvironmentFilePath, BuzzRelayBindAddress, BuzzRelayHealthPort, BuzzRelayRedisURL, BuzzRelayLocalURL, BuzzRelayImportOverrideEnvPath, BuzzRelayBinaryPath)
}

func BuzzRelayHealthCheckCommand() string {
	return "curl --max-time 5 -fsS http://" + BuzzRelayBindAddress + "/_readiness >/dev/null && echo ok || echo no"
}

func ChatdServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz chatd bridge
After=network-online.target buzz-relay.service internkim-admind.service
Wants=network-online.target

[Service]
User=root
EnvironmentFile=%s
Environment=CHATD_BOT_USER_NAME=%s
Environment=CHATD_BUZZ_RELAY_URL=ws://%s
Environment=CHATD_BUZZ_ACCOUNT_LINKS_PATH=%s
Environment=CHATD_LISTEN_PORT=%s
Environment=CHATD_ADMIND_BASE_URL=%s
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, ChatdEnvironmentFilePath, ChatdBotUserName, BuzzRelayBindAddress, BuzzAccountLinksPath, ChatdListenPort, AdmindBaseURL, ChatdBinaryPath)
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
Description=InternKim llama.cpp Server
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
Description=InternKim llama.cpp Embedding Server
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
