package blueclaw

import (
	"fmt"

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

func CapabilitydServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=InternKim Capability Daemon
After=network-online.target time-sync.target mattermost.service
Wants=network-online.target time-sync.target

[Service]
User=root
RuntimeDirectory=internkim
ExecStart=%s --vsock-port %d --companion-url http://127.0.0.1:18080/_internkim/companion
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, CapabilitydBinaryPath, CapabilityVSockPort)
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
ExecStart=%s -m %s --host %s --port %s -ngl 99 --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppModelPath, locallm.LlamaCppHost, locallm.LlamaCppPort)
}

func LlamaCppEmbeddingServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=InternKim llama.cpp Embedding Server
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=root
Environment=LD_LIBRARY_PATH=%s
ExecStart=%s -m %s --host %s --port %s -ngl 99 --embeddings --pooling mean --log-disable
Restart=on-failure
RestartSec=2
TimeoutStartSec=120

[Install]
WantedBy=multi-user.target
`, locallm.LlamaCppLibraryDir, locallm.LlamaCppBinaryPath, locallm.LlamaCppEmbeddingModelPath, locallm.LlamaCppHost, locallm.LlamaCppEmbeddingPort)
}
