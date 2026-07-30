package setup

import (
	"fmt"

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
const buzzRelayPublicURLDropInPath = "/etc/systemd/system/buzz-relay.service.d/public-url.conf"

// StepBuzzPublicHost makes the loopback relay reachable at its public host P
// (derived from the device URL). Internal clients resolve P to loopback via
// /etc/hosts, terminate TLS at a stunnel4 daemon on 127.0.0.1:443, and reach
// the relay at 127.0.0.1:3000. The community row is re-keyed to P so the relay
// advertises the public host. Every action is idempotent.
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
		publicHost := blueclaw.DeriveRelayPublicHost(context.PublicURL)
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
		return trimmedRun(context, "systemctl is-active "+buzzRelayStunnelServiceName) == "active"
	},
	Run: func(context *Context) error {
		if context.Backend != BackendSSH {
			return nil
		}
		publicHost := blueclaw.DeriveRelayPublicHost(context.PublicURL)
		if publicHost == "" {
			return nil
		}
		connection := context.SSH
		connection.Run(buzzHostsAliasCommand(publicHost))
		connection.Run(buzzRelayCertificateCommand(publicHost))
		connection.Run(buzzRelayStunnelCommand())
		connection.Run("systemctl stop " + blueclaw.BuzzRelayServiceName)
		connection.Run(buzzCommunityRekeyCommand(publicHost))
		connection.Run(buzzRelayPublicURLDropInCommand(publicHost))
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

func buzzRelayStunnelCommand() string {
	return `command -v stunnel4 >/dev/null 2>&1 || DEBIAN_FRONTEND=noninteractive apt-get install -y stunnel4
systemctl disable --now stunnel4 2>/dev/null || true
pkill -f 'stunnel.*buzz-relay.conf' 2>/dev/null || true
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

func buzzCommunityRekeyCommand(publicHost string) string {
	return `if su - postgres -c "psql -d ` + blueclaw.BuzzRelayDatabaseName + ` -tAc \"SELECT to_regclass('public.communities')\"" 2>/dev/null | grep -q communities; then
  su - postgres -c "psql -d ` + blueclaw.BuzzRelayDatabaseName + ` -c \"UPDATE communities SET host='` + publicHost + `' WHERE host='` + blueclaw.BuzzRelayBindAddress + `' AND NOT EXISTS (SELECT 1 FROM communities WHERE lower(host)=lower('` + publicHost + `'))\""
fi`
}

func buzzRelayPublicURLDropInCommand(publicHost string) string {
	return `mkdir -p ` + buzzRelayServiceDropInDirectory + `
printf '[Service]\nEnvironment=RELAY_URL=wss://%s\n' '` + publicHost + `' > ` + buzzRelayPublicURLDropInPath + `
systemctl daemon-reload`
}
