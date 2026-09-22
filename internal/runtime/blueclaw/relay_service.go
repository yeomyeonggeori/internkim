package blueclaw

import "fmt"

// RelayServiceUnit is the unit host/relay/internkim-relay.service documents for
// a self-hosted install, with the paths this deployment uses filled in.
// TestTheShippedRelayUnitMatchesTheDocumentedOne fails when the two drift.
func RelayServiceUnit() string {
	return relayServiceUnit(RelayBinaryPath, RelayStateDirectoryName)
}

// The packaged company host runs the same relay from the path dpkg is allowed to
// write, and keeps its state beside the company tree rather than inside it. Two
// values of one declaration, and the reason is in RelayStateDirectoryName.
func relayServiceUnit(binaryPath string, stateDirectoryName string) string {
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
Environment=RELAY_STATE_DIR=%s
Restart=always
RestartSec=30s
User=%s
Group=%s
StateDirectory=%s
StateDirectoryMode=0750
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true

[Install]
WantedBy=multi-user.target
`, RelayEnvironmentFilePath, binaryPath, RelayEnvironmentFilePath,
		RelayStateDirectoryPath(stateDirectoryName), RelayUserName, RelayUserName, stateDirectoryName)
}

// RelayStateDirectoryPath is where systemd's StateDirectory= lands, named in the
// unit so the relay and its supervisor cannot disagree about it.
func RelayStateDirectoryPath(stateDirectoryName string) string {
	return "/var/lib/" + stateDirectoryName
}
