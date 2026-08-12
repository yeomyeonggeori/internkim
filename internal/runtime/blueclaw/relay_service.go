package blueclaw

import "fmt"

// RelayServiceUnit is the unit host/relay/internkim-relay.service documents for
// a self-hosted install, with the paths this deployment uses filled in.
// TestTheShippedRelayUnitMatchesTheDocumentedOne fails when the two drift.
func RelayServiceUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim relay
Documentation=https://github.com/yeomyeonggeori/internkim/blob/main/host/README.md
After=network-online.target
Wants=network-online.target
ConditionPathExists=%s
StartLimitIntervalSec=600
StartLimitBurst=5

[Service]
ExecStart=%s
EnvironmentFile=%s
Restart=always
RestartSec=30s
User=%s
Group=%s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
`, RelayEnvironmentFilePath, RelayBinaryPath, RelayEnvironmentFilePath, RelayUserName, RelayUserName)
}
