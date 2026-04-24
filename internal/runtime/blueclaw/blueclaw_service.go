package blueclaw

import "fmt"

func BlueclawServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Blueclaw
After=network-online.target time-sync.target
Wants=network-online.target time-sync.target

[Service]
User=%s
EnvironmentFile=%s
Environment=HOME=%s
Environment=BLUECLAW_SESSIONS_DIR=%s
Environment=MM_URL_FILE=%s
Environment=MM_BOT_TOKEN_FILE=%s
Environment=SLACK_BOT_TOKEN_FILE=%s
Environment=RUST_LOG=%s
ExecStart=%s -runtime %s -policy %s
Restart=on-failure

[Install]
WantedBy=multi-user.target
`, BlueclawUser, BlueclawOpenRouterEnvFile, BlueclawHomePath, BlueclawSessionDirectory, BlueclawMattermostURLPath, BlueclawMattermostTokenPath, BlueclawSlackTokenPath, BlueclawRuntimeLogLevel, BlueclawBinaryPath, BlueclawRuntimeConfigPath, BlueclawPolicyConfigPath)
}
