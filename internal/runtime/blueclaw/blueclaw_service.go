package blueclaw

import "fmt"

func BlueclawServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Blueclaw
After=network-online.target time-sync.target postgresql.service graphiti-memoryd.service
Wants=network-online.target time-sync.target postgresql.service graphiti-memoryd.service

[Service]
User=%s
Environment=HOME=%s
Environment=BLUECLAW_SESSIONS_DIR=%s
Environment=RUST_LOG=%s
ExecStart=%s -runtime %s -policy %s
Restart=on-failure

[Install]
WantedBy=multi-user.target
`, BlueclawUser, BlueclawHomePath, BlueclawSessionDirectory, BlueclawRuntimeLogLevel, BlueclawBinaryPath, BlueclawRuntimeConfigPath, BlueclawPolicyConfigPath)
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
ExecStart=%s
Restart=on-failure
RestartSec=2

[Install]
WantedBy=multi-user.target
`, CapabilitydBinaryPath)
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
