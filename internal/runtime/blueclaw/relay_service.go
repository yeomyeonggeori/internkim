package blueclaw

import "fmt"

// RelayServiceUnit is the unit host/relay/internkim-relay.service documents for
// a self-hosted install, with the paths this deployment uses filled in.
// TestTheShippedRelayUnitMatchesTheDocumentedOne fails when the two drift.
func RelayServiceUnit() string {
	return relayServiceUnit(RelayBinaryPath)
}

// The packaged company host runs the same relay from the path dpkg is allowed to
// write, and nothing else about the unit changes.
func relayServiceUnit(binaryPath string) string {
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
StateDirectory=internkim/relay
StateDirectoryMode=0750
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
`, RelayEnvironmentFilePath, binaryPath, RelayEnvironmentFilePath, RelayUserName, RelayUserName)
}
