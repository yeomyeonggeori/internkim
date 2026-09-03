package setup

import (
	"fmt"
	"strings"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const buzzRelayKeyPath = "/root/.internkim/tls/relay.key"
const buzzRelayTLSDirectory = "/root/.internkim/tls"
const buzzRelayTrustStorePath = "/usr/local/share/ca-certificates/buzz-relay.crt"
const buzzRelayStunnelConfigurationPath = "/etc/stunnel/buzz-relay.conf"
const buzzRelayStunnelServiceUnitPath = "/etc/systemd/system/buzz-relay-stunnel.service"
const buzzRelayStunnelServiceName = "buzz-relay-stunnel"
const buzzRelayStunnelAcceptAddress = "127.0.0.1:443"
const buzzRelayServiceDropInDirectory = "/etc/systemd/system/buzz-relay.service.d"
const buzzRelayEnvironmentDirectory = "/root/.internkim/env"
const buzzRelayPublicURLDropInPath = "/etc/systemd/system/buzz-relay.service.d/public-url.conf"

// StepBuzzPublicHost makes the loopback relay reachable at the public host P the
// company chose for it. Internal clients resolve P to loopback via /etc/hosts,
// terminate TLS at a stunnel4 daemon on 127.0.0.1:443, and reach the relay at
// 127.0.0.1:3000. The community row is re-keyed to P so the relay advertises the
// public host, and P is recorded so admind presents the same name.
//
// Without a chosen domain the step does nothing: the relay stays on loopback,
// which is where a company that reaches its messenger through the central plane
// wants it.
var StepBuzzPublicHost = Step{
	Name: "buzz-public-host",
	Deps: []string{"buzz-relay"},
	Title: func(context *Context) string {
		return context.T("Buzz 공개 호스트 구성 중...", "Configuring Buzz public host...")
	},
	IsSatisfied: func(context *Context) bool {
		if context.Backend != BackendSSH {
			return true
		}
		publicHost := blueclaw.RelayPublicHost(context.RelayDomain)
		if publicHost == "" {
			return true
		}
		if !buzzHostsAliasIsPresent(context, publicHost) {
			return false
		}
		if !sshFileExists(context, blueclaw.BuzzRelayCertificatePath) {
			return false
		}
		if !sshFileExists(context, buzzRelayPublicURLDropInPath) {
			return false
		}
		if buzzRelayRecordedPublicHost(context) != publicHost {
			return false
		}
		return trimmedRun(context, "systemctl is-active "+buzzRelayStunnelServiceName) == "active"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		publicHost := blueclaw.RelayPublicHost(context.RelayDomain)
		if publicHost == "" {
			return nil
		}
		previousHost := buzzRelayRecordedPublicHost(context)
		connection := context.SSH
		connection.Run(buzzHostsAliasCommand(publicHost))
		connection.Run(buzzRelayCertificateCommand(publicHost))
		if errorValue := installBuzzRelayStunnel(connection); errorValue != nil {
			return errorValue
		}
		connection.Run("systemctl stop " + blueclaw.BuzzRelayServiceName)
		connection.Run(buzzCommunityRekeyCommand(publicHost, previousHost))
		connection.Run(buzzRelayPublicURLDropInCommand(publicHost))
		connection.Run(buzzRelayPublicURLRecordCommand(publicHost))
		connection.Run("systemctl start " + blueclaw.BuzzRelayServiceName)

		fmt.Println("  " + context.T("Buzz 공개 호스트 구성 완료", "Buzz public host configured"))
		return nil
	},
	RunSD: func(context *Context) error {
		return nil
	},
}

func buzzHostsAliasIsPresent(context *Context, publicHost string) bool {
	return trimmedRun(context, "grep -qF '127.0.0.1 "+publicHost+"' /etc/hosts && echo y || echo n") == "y"
}

func buzzHostsAliasCommand(publicHost string) string {
	return `grep -qF '127.0.0.1 ` + publicHost + `' /etc/hosts || printf '127.0.0.1 %s\n' '` + publicHost + `' >> /etc/hosts`
}

func buzzRelayCertificateCommand(publicHost string) string {
	return `mkdir -p ` + buzzRelayTLSDirectory + `
chmod 700 ` + buzzRelayTLSDirectory + `
if [ ! -f ` + blueclaw.BuzzRelayCertificatePath + ` ] || ! openssl x509 -in ` + blueclaw.BuzzRelayCertificatePath + ` -noout -ext subjectAltName 2>/dev/null | grep -qF "DNS:` + publicHost + `"; then
  openssl req -x509 -newkey rsa:2048 -nodes -keyout ` + buzzRelayKeyPath + ` -out ` + blueclaw.BuzzRelayCertificatePath + ` -days 3650 -subj "/CN=` + publicHost + `" -addext "subjectAltName=DNS:` + publicHost + `"
  cp ` + blueclaw.BuzzRelayCertificatePath + ` ` + buzzRelayTrustStorePath + `
  update-ca-certificates
elif [ ! -f ` + buzzRelayTrustStorePath + ` ]; then
  cp ` + blueclaw.BuzzRelayCertificatePath + ` ` + buzzRelayTrustStorePath + `
  update-ca-certificates
fi`
}

func installBuzzRelayStunnel(connection BoardConnection) error {
	output := connection.Run(withPackageWorkSettled(buzzRelayStunnelCommand()))
	if strings.TrimSpace(connection.Run("command -v stunnel4 >/dev/null 2>&1 && echo installed || echo missing")) == "installed" {
		return nil
	}
	return fmt.Errorf("stunnel4 is not installed, so the TLS terminator the relay is reached through cannot exec: %s", strings.TrimSpace(output))
}

func buzzRelayStunnelCommand() string {
	return `command -v stunnel4 >/dev/null 2>&1 || DEBIAN_FRONTEND=noninteractive apt-get install -y -qq stunnel4 2>&1 | tail -5
systemctl disable --now stunnel4 2>/dev/null || true
pkill -x stunnel4 2>/dev/null || true
cat > ` + buzzRelayStunnelConfigurationPath + ` <<'STUNNELCONFEOF'
foreground = yes
[buzz-relay]
accept = ` + buzzRelayStunnelAcceptAddress + `
connect = ` + blueclaw.BuzzRelayBindAddress + `
cert = ` + blueclaw.BuzzRelayCertificatePath + `
key = ` + buzzRelayKeyPath + `
STUNNELCONFEOF
cat > ` + buzzRelayStunnelServiceUnitPath + ` <<'STUNNELUNITEOF'
[Unit]
Description=Buzz relay TLS terminator (stunnel)
After=network-online.target ` + blueclaw.BuzzRelayServiceName + `.service
Wants=network-online.target
[Service]
ExecStart=/usr/bin/stunnel4 ` + buzzRelayStunnelConfigurationPath + `
Restart=always
RestartSec=2
[Install]
WantedBy=multi-user.target
STUNNELUNITEOF
systemctl daemon-reload
systemctl enable ` + buzzRelayStunnelServiceName + `
systemctl restart ` + buzzRelayStunnelServiceName + ``
}

// The community the relay serves is keyed by the Host header, so it has to
// follow the public host. A relay that has never had one is keyed by its bind
// address; one that is being moved to a new domain is keyed by the name this
// step recorded last time, which is how a rename reaches the row that a company
// created rather than any other.
func buzzCommunityRekeyCommand(publicHost string, previousHost string) string {
	rekeyedFrom := "'" + blueclaw.BuzzRelayBindAddress + "'"
	if previousHost != "" && previousHost != publicHost {
		rekeyedFrom += ",'" + previousHost + "'"
	}
	return `if su - postgres -c "psql -d ` + blueclaw.BuzzRelayDatabaseName + ` -tAc \"SELECT to_regclass('public.communities')\"" 2>/dev/null | grep -q communities; then
  su - postgres -c "psql -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"UPDATE communities SET host='` + publicHost + `' WHERE host IN (` + rekeyedFrom + `) AND NOT EXISTS (SELECT 1 FROM communities WHERE lower(host)=lower('` + publicHost + `'))\""
fi`
}

func buzzRelayPublicURLDropInCommand(publicHost string) string {
	return `mkdir -p ` + buzzRelayServiceDropInDirectory + `
printf '[Service]\nEnvironment=RELAY_URL=wss://%s\n' '` + publicHost + `' > ` + buzzRelayPublicURLDropInPath + `
systemctl daemon-reload`
}

func buzzRelayRecordedPublicHost(context *Context) string {
	return blueclaw.RelayPublicHost(trimmedRun(context, "cat "+blueclaw.BuzzRelayPublicURLFilePath+" 2>/dev/null"))
}

func buzzRelayPublicURLRecordCommand(publicHost string) string {
	return `mkdir -p ` + buzzRelayEnvironmentDirectory + `
printf 'wss://%s' '` + publicHost + `' > ` + blueclaw.BuzzRelayPublicURLFilePath + `
chmod 640 ` + blueclaw.BuzzRelayPublicURLFilePath
}
