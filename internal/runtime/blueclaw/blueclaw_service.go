package blueclaw

import "fmt"

func BlueclawServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Blueclaw
After=network-online.target time-sync.target postgresql.service
Wants=network-online.target time-sync.target postgresql.service

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
