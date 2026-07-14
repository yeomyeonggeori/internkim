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
ExecStart=%s -companion-url http://127.0.0.1:18080/_internkim/companion
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, BlueclawUser, BlueclawHomePath, GraphitiKuzuPath, GraphitiMemorydPath)
}

func SDKDServiceUnit() string {
	return SDKDServiceUnitForLocalOnly(LocalOnlyEnabled())
}

func SDKDServiceUnitForLocalOnly(isLocalOnly bool) string {
	localOnlyValue := 0
	openRouterEnvironment := "Environment=OPENROUTER_API_KEY_PATH=" + SDKDRuntimeOpenRouterKeyPath + "\n"
	networkPolicy := ""
	if isLocalOnly {
		localOnlyValue = 1
		openRouterEnvironment = ""
		networkPolicy = "IPAddressDeny=any\nIPAddressAllow=localhost\n"
	}
	authCredentialPreStart, openRouterCredentialPreStart := sdkdCredentialPreStartCommands(isLocalOnly)
	return fmt.Sprintf(`[Unit]
Description=Blueclaw AI SDK Runtime
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
DynamicUser=yes
RuntimeDirectory=blueclaw-sdkd
RuntimeDirectoryMode=0700
UMask=0077
Environment=BLUECLAW_SDKD_SOCKET_PATH=%s
Environment=BLUECLAW_SDKD_LLAMA_BASE_URL=%s
Environment=BLUECLAW_SDKD_LLAMA_MODEL=local/gemma-4-E2B-it-qat-UD-Q4_K_XL
Environment=BLUECLAW_SDKD_LLAMA_STRUCTURED_OUTPUTS_ENABLED=true
Environment=BLUECLAW_SDKD_LOCAL_ONLY=%d
Environment=BLUECLAW_SDKD_AUTH_KEY_PATH=%s
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
`, SDKDSocketPath, locallm.LlamaCppBaseURL, localOnlyValue, SDKDRuntimeAuthKeyPath, openRouterEnvironment, authCredentialPreStart, openRouterCredentialPreStart, SDKDBinaryPath, networkPolicy)
}

func sdkdCredentialPreStartCommands(isLocalOnly bool) (string, string) {
	authCredentialCommand := fmt.Sprintf(
		"set -eu; install -m 0400 %s %s; chown --reference=%s %s",
		SDKDServiceAuthKeyPath,
		SDKDRuntimeAuthKeyPath,
		SDKDRuntimeDirectoryPath,
		SDKDRuntimeAuthKeyPath,
	)
	if isLocalOnly {
		return sdkdPrivilegedPreStart(authCredentialCommand), sdkdPrivilegedPreStart("rm -f " + SDKDRuntimeOpenRouterKeyPath)
	}
	openRouterCredentialCommand := fmt.Sprintf(
		"set -eu; if [ -s %s ]; then install -m 0400 %s %s; chown --reference=%s %s; else rm -f %s; fi",
		SDKDServiceOpenRouterKeyPath,
		SDKDServiceOpenRouterKeyPath,
		SDKDRuntimeOpenRouterKeyPath,
		SDKDRuntimeDirectoryPath,
		SDKDRuntimeOpenRouterKeyPath,
		SDKDRuntimeOpenRouterKeyPath,
	)
	return sdkdPrivilegedPreStart(authCredentialCommand), sdkdPrivilegedPreStart(openRouterCredentialCommand)
}

func sdkdPrivilegedPreStart(command string) string {
	return "ExecStartPre=+/bin/sh -c '" + command + "'\n"
}

func SDKDServiceCredentialInstallCommand(isLocalOnly bool) string {
	openRouterCredentialCommand := "if [ -s " + OpenRouterKeyPath + " ]; then\n" +
		"  install -o root -g root -m 600 " + OpenRouterKeyPath + " " + SDKDServiceOpenRouterKeyPath + "\n" +
		"else\n" +
		"  rm -f " + SDKDServiceOpenRouterKeyPath + "\n" +
		"fi"
	if isLocalOnly {
		openRouterCredentialCommand = "rm -f " + SDKDServiceOpenRouterKeyPath
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
%[5]s`, filepath.Dir(SDKDAuthKeyPath), SDKDAuthKeyPath, SDKDServiceCredentialDirectoryPath, SDKDServiceAuthKeyPath, openRouterCredentialCommand)
}

func CapabilitydServiceUnit() string {
	return CapabilitydServiceUnitForLocalInferenceMode("")
}

func CapabilitydServiceUnitForLocalInferenceMode(localInferenceMode string) string {
	return fmt.Sprintf(`[Unit]
Description=InternKim Capability Daemon
After=network-online.target time-sync.target mattermost.service blueclaw-sdkd.service
Wants=network-online.target time-sync.target blueclaw-sdkd.service

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
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, AdmindBinaryPath)
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
ExecStart=%s -m %s --host %s --port %s -ngl 0 --embeddings --pooling mean --batch-size %s --ubatch-size %s --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppEmbeddingModelPath, locallm.LlamaCppHost, locallm.LlamaCppEmbeddingPort, locallm.LlamaCppEmbeddingBatchSize, locallm.LlamaCppEmbeddingUBatchSize)
}
