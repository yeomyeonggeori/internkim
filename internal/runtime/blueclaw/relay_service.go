package blueclaw

import (
	"fmt"
	"strings"
)

// RelayServiceUnit is the unit host/relay/internkim-relay.service documents for
// a self-hosted install, with the paths this deployment uses filled in.
// TestTheShippedRelayUnitMatchesTheDocumentedOne fails when the two drift.
func RelayServiceUnit() string {
	stateSetting := setting("RELAY_STATE_DIR", RelayStateDirectoryPath(RelayStateDirectoryName))
	return relayServiceUnit(RelayBinaryPath, RelayStateDirectoryName, []EnvironmentSetting{stateSetting})
}

// The packaged company host runs the same relay from the path dpkg is allowed to
// write, keeps its state beside the company tree rather than inside it, and
// carries the settings its package decides. The reason for the state directory
// is in RelayStateDirectoryName.
func relayServiceUnit(binaryPath string, stateDirectoryName string, settings []EnvironmentSetting) string {
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
%sRestart=always
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
		systemdEnvironmentLines(settings), RelayUserName, RelayUserName, stateDirectoryName)
}

// RelayStateDirectoryPath is where systemd's StateDirectory= lands, named in the
// unit so the relay and its supervisor cannot disagree about it.
func RelayStateDirectoryPath(stateDirectoryName string) string {
	return "/var/lib/" + stateDirectoryName
}

func systemdEnvironmentLines(settings []EnvironmentSetting) string {
	lines := &strings.Builder{}
	for _, value := range settings {
		lines.WriteString("Environment=" + value.Name + "=" + value.Value + "\n")
	}
	return lines.String()
}
