package blueclaw

import (
	"fmt"
	"strings"
)

// The company host installed by dpkg is the same set of processes host/entrypoint.sh
// starts, supervised by systemd instead of by a shell. It is not the device, and four
// of the seven services differ from their device units in ways no argument bridges:
// blueclaw runs its own binary here and a Cloud Hypervisor guest there, capabilityd is
// reached over a unix socket here and a vsock there, admind carries the central plane's
// addresses here and none there, and dpkg may not write /usr/local, so every path moves
// to /usr/bin. What is shared is shared: the relay unit is the one renderer with the
// binary path passed in, and every name, port and address below is the constant the
// device path already uses.
const (
	CompanyPackageName          = "internkim"
	CompanyPackageMaintainer    = "internkim <support@intern.kim>"
	CompanyPackageVendor        = "yeomyeonggeori"
	CompanyPackageHomepage      = "https://intern.kim"
	CompanyPackageSection       = "admin"
	CompanyPackageBinaryRoot    = "/usr/bin"
	CompanyPackageLibraryRoot   = "/opt/internkim"
	CompanyPackageHelperRoot    = "/usr/lib/internkim"
	CompanyPackageUnitRoot      = "/usr/lib/systemd/system"
	CompanyPackageSkillsPath    = "/opt/internkim/skills"
	CompanyPackageTemplatePath  = "/opt/internkim/runtime.template.json"
	CompanyPackageMigrationPath = "/opt/blueclaw/migrations"
	CompanyPackagePreparePath   = "/usr/lib/internkim/prepare-company-host"

	// POSIXHelperProgramName is what lets the unprivileged agent act as the
	// person who asked, and it is the one setuid file the package ships. Debian
	// forbids a package writing /usr/local, where the device path keeps it, so
	// the packaged host names its own path and the rendered runtime document
	// carries that name rather than the device's.
	POSIXHelperProgramName     = "blueclaw-posix-helper"
	CompanyHostPOSIXHelperPath = "/usr/lib/internkim/" + POSIXHelperProgramName

	// The company directory is keyed by company id, and a unit rendered at package
	// build time cannot name an id nobody has chosen yet. `internkim install` points
	// this symlink at the company it prepared, and every unit reads through it.
	CompanyHostStateRoot        = "/var/lib/internkim"
	CompanyHostCompaniesRoot    = "/var/lib/internkim/companies"
	CompanyHostCurrentPath      = "/var/lib/internkim/current"
	CompanyHostEnvironmentPath  = "/var/lib/internkim/current/host.env"
	CompanyHostSecretsRoot      = "/var/lib/internkim/current/secrets"
	CompanyHostAgentKeyPath     = "/var/lib/internkim/current/secrets/agent-key"
	CompanyHostModelKeyPath     = "/var/lib/internkim/current/secrets/openrouter-key"
	CompanyHostIdentitySeedPath = "/var/lib/internkim/current/secrets/buzz-key-seed"
	CompanyHostBuzzDatabasePath = "/var/lib/internkim/current/secrets/buzz-database.env"
	CompanyHostBuzzRelayKeyPath = "/var/lib/internkim/current/secrets/buzz-relay.env"
	CompanyHostRelayMediaPath   = "/var/lib/internkim/current/secrets/buzz-media.env"
	CompanyHostMediaRootPath    = "/var/lib/internkim/media"
	CompanyHostMediaSecretPath  = "/var/lib/internkim/current/secrets/buzz-media-root.env"
	CompanyHostMediaBucketPath  = "/var/lib/internkim/media/" + BuzzMediaBucket
	CompanyHostChatdSecretPath  = "/var/lib/internkim/current/secrets/chatd.env"
	CompanyHostAccountLinksPath = "/var/lib/internkim/buzz-account-links.json"
	CompanyHostWorkspacePath    = "/workspace"
	CompanyHostLogPath          = "/var/log/internkim"
	CompanyHostBrowserStatePath = "/var/lib/internkim-moli"
	CompanyHostRunPath          = "/run/internkim"
	CompanyHostRunSecretsPath   = "/run/internkim/secrets"
	CompanyHostRunAgentKeyPath  = "/run/internkim/secrets/agent-key"
	CompanyHostRunModelKeyPath  = "/run/internkim/secrets/openrouter-key"
	CompanyHostRuntimeDocument  = "/run/internkim/runtime.json"
	CompanyHostPolicyDocument   = "/run/internkim/policy.json"
	CompanyHostACPSocketPath    = "/run/internkim/blueclaw-acp.sock"

	CompanyHostConfigurationRoot = "/etc/internkim"

	// The state root has one mode and every writer takes it from here. It is
	// 0700 root:root because a company's identity on the plane and the seed that
	// signs a message under a person's own name sit under it, and because
	// nothing unprivileged has business inside: the one service that runs as an
	// ordinary account keeps its state in a sibling directory, which is what
	// CompanyHostRelayStateDirectoryName is for. The package creates it, the
	// prepare service re-creates it, and `internkim install` creates it on a box
	// that never saw a package; all three name this.
	CompanyHostStateRootMode = 0o700

	// The one file an operator is expected to open. Every value in it is the
	// default the unit would use anyway, and it is the single place the object
	// store is named: BUZZ_S3_ENDPOINT, BUZZ_S3_BUCKET and BUZZ_S3_REGION point
	// the messenger's media at whatever serves S3 on this box. Keys stay out of
	// it, in the company directory, because dpkg keeps a conffile readable.
	CompanyHostSettingsPath = "/etc/internkim/company-host.env"

	// An override the container reads from /etc/blueclaw keeps the same meaning
	// here: present, it wins over the rendered template; absent, nothing happens.
	CompanyHostRuntimeOverridePath = "/etc/internkim/runtime.json"
	CompanyHostPolicyOverridePath  = "/etc/internkim/policy.json"

	CompanyHostPrepareServiceName  = "internkim-prepare"
	CompanyHostMaildServiceName    = "internkim-maild"
	CompanyHostMaildListenAddress  = "127.0.0.1:18092"
	CompanyHostAdmindListenAddress = "127.0.0.1:18080"
	CompanyHostChatdEndpoint       = "http://127.0.0.1:18090"
	CompanyHostArrivalsInboundURL  = "http://127.0.0.1:18091/inbound"
	CompanyHostBrowserFirstPort    = "9230"
	CompanyHostBrowserCapacity     = "4"
)

// CompanyPackageBinaryPath is where the package puts a program it ships. Debian
// policy forbids a package writing /usr/local, which is also what keeps a packaged
// install from colliding with the device path's own binaries during convergence.
func CompanyPackageBinaryPath(programName string) string {
	return CompanyPackageBinaryRoot + "/" + programName
}

// CompanyPackageUnit is one systemd unit the package installs.
type CompanyPackageUnit struct {
	Name     string
	Contents string
}

func (unit CompanyPackageUnit) FileName() string {
	return unit.Name + ".service"
}

func (unit CompanyPackageUnit) InstalledPath() string {
	return CompanyPackageUnitRoot + "/" + unit.FileName()
}

// CompanyPackageUnits is every unit the package installs, in start order. Each one
// carries ConditionPathExists over a file `internkim install` writes, so a box that
// has the package but not yet a company sits inactive rather than restarting into a
// failure it cannot explain.
func CompanyPackageUnits() []CompanyPackageUnit {
	return []CompanyPackageUnit{
		{Name: RelayServiceName, Contents: relayServiceUnit(CompanyPackageBinaryPath(RelayName), CompanyHostRelayStateDirectoryName)},
		{Name: CompanyHostPrepareServiceName, Contents: companyHostPrepareUnit()},
		{Name: BuzzMediaServiceName, Contents: companyHostBuzzMediaUnit()},
		{Name: BuzzRelayServiceName, Contents: companyHostBuzzRelayUnit()},
		{Name: CapabilitydServiceName, Contents: companyHostCapabilitydUnit()},
		{Name: BlueclawServiceName, Contents: companyHostBlueclawUnit()},
		{Name: AdmindServiceName, Contents: companyHostAdmindUnit()},
		{Name: CompanyHostMaildServiceName, Contents: companyHostMaildUnit()},
		{Name: ChatdServiceName, Contents: companyHostChatdUnit()},
	}
}

// companyHostCommonService is what every unit of the bundle shares: the company's
// own environment, then the operator's file last so an edited setting wins over a
// rendered default.
func companyHostCommonService() string {
	return "EnvironmentFile=" + CompanyHostEnvironmentPath + "\n" +
		"EnvironmentFile=-" + CompanyHostSettingsPath + "\n" +
		"Restart=on-failure\nRestartSec=5\n"
}

func companyHostPrepareUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim company host preparation
Documentation=https://github.com/yeomyeonggeori/internkim/blob/main/docs/internal/native-packaging.md
After=network-online.target
Wants=network-online.target
ConditionPathExists=%s

[Service]
Type=oneshot
RemainAfterExit=yes
EnvironmentFile=%s
EnvironmentFile=-%s
ExecStart=%s

[Install]
WantedBy=multi-user.target
`, CompanyHostAgentKeyPath, CompanyHostEnvironmentPath, CompanyHostSettingsPath, CompanyPackagePreparePath)
}

// The messenger is the service that must not run without PostgreSQL, and systemd has
// no health gate: BindsTo stops it when the database unit stops, and Restart=on-failure
// is what waits for a database that is up but not yet accepting connections.
func companyHostBuzzRelayUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz Relay
After=network-online.target postgresql.service redis-server.service %s.service
Wants=network-online.target redis-server.service %s.service
BindsTo=postgresql.service
ConditionPathExists=%s

[Service]
User=root
EnvironmentFile=%s
EnvironmentFile=%s
EnvironmentFile=-%s
Environment=BUZZ_BIND_ADDR=%s
Environment=BUZZ_HEALTH_PORT=%s
Environment=REDIS_URL=%s
Environment=RELAY_URL=%s
Environment=BUZZ_AUTO_MIGRATE=1
Environment=BUZZ_REQUIRE_RELAY_MEMBERSHIP=true
Environment=BUZZ_DB_POOL_SIZE=50
%sExecStart=%s
KillMode=mixed
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
`,
		BuzzMediaServiceName,
		BuzzMediaServiceName,
		CompanyHostBuzzRelayKeyPath,
		CompanyHostBuzzRelayKeyPath,
		CompanyHostBuzzDatabasePath,
		CompanyHostRelayMediaPath,
		BuzzRelayBindAddress,
		BuzzRelayHealthPort,
		BuzzRelayRedisURL,
		BuzzRelayLocalURL,
		companyHostCommonService(),
		CompanyPackageBinaryPath(BuzzRelayName))
}

// The posix backend serves every directory under its root as a bucket, so the bucket
// is a directory dpkg creates rather than something an S3 client makes. --versioning-dir
// is absent for the reason BuzzMediaServiceUnit gives.
func companyHostBuzzMediaUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz Media Store
After=network-online.target
Wants=network-online.target
ConditionPathExists=%s

[Service]
User=root
EnvironmentFile=%s
%sExecStart=%s --port %s --health %s posix %s

[Install]
WantedBy=multi-user.target
`,
		CompanyHostMediaSecretPath,
		CompanyHostMediaSecretPath,
		companyHostCommonService(),
		CompanyPackageBinaryPath(BuzzMediaProgramName),
		BuzzMediaAddress,
		BuzzMediaHealthPath,
		CompanyHostMediaRootPath)
}

func companyHostCapabilitydUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim Capability Daemon
After=network-online.target %s.service
Requires=%s.service
Wants=network-online.target
ConditionPathExists=%s

[Service]
User=root
%sExecStart=%s --socket %s --openrouter-key %s --local-inference-mode remote --blueclaw-url %s --admind-url http://%s --chatd-endpoint %s --chatd-platform ${MESSENGER_PLATFORM} --device-browser %s --device-browser-state-dir %s --device-browser-first-port %s --device-browser-capacity %s --device-browser-user %s

[Install]
WantedBy=multi-user.target
`,
		CompanyHostPrepareServiceName,
		CompanyHostPrepareServiceName,
		CompanyHostAgentKeyPath,
		companyHostCommonService(),
		CompanyPackageBinaryPath(CapabilitydName),
		CapabilitySocketPath,
		CompanyHostModelKeyPath,
		BlueclawBaseURL,
		CompanyHostAdmindListenAddress,
		CompanyHostChatdEndpoint,
		CompanyPackageBinaryPath(DeviceBrowserName),
		CompanyHostBrowserStatePath,
		CompanyHostBrowserFirstPort,
		CompanyHostBrowserCapacity,
		BlueclawUser)
}

func companyHostBlueclawUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Blueclaw
After=network-online.target postgresql.service %s.service %s.service
Requires=%s.service
Wants=network-online.target %s.service
BindsTo=postgresql.service
ConditionPathExists=%s

[Service]
User=%s
Group=%s
WorkingDirectory=%s
Environment=HOME=%s
Environment=BLUECLAW_BUNDLED_SKILLS_PATH=%s
%sExecStart=%s -runtime %s -policy %s -acp-socket %s -inbound acp
KillMode=mixed
TimeoutStopSec=30

[Install]
WantedBy=multi-user.target
`,
		CompanyHostPrepareServiceName,
		CapabilitydServiceName,
		CompanyHostPrepareServiceName,
		CapabilitydServiceName,
		CompanyHostRuntimeDocument,
		BlueclawUser,
		BlueclawUser,
		CompanyHostWorkspacePath,
		BlueclawHomePath,
		CompanyPackageSkillsPath,
		companyHostCommonService(),
		CompanyPackageBinaryPath(BlueclawName),
		CompanyHostRuntimeDocument,
		CompanyHostPolicyDocument,
		CompanyHostACPSocketPath)
}

// admind reconciles the company's roster onto blueclaw as it starts, so it comes
// after it: a reconcile against a blueclaw that is not up yet waits two minutes for
// the next one, and for those two minutes nobody the company knows resolves.
func companyHostAdmindUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim Admin Gateway
After=network-online.target %s.service
Requires=%s.service
Wants=network-online.target %s.service
ConditionPathExists=%s

[Service]
User=root
%sExecStart=%s -listen %s -capability-socket %s -chatd-endpoint %s -chatd-platform ${MESSENGER_PLATFORM} -blueclaw-url %s -blueclaw-policy %s -buzz-key-seed-path %s -buzz-database-url-path %s -buzz-relay-key-path %s -buzz-admin-command %s -buzz-relay-url %s -buzz-account-links %s -site-scaffold %s/website/assets/scaffold/app -central-plane-app-url ${INTERNKIM_APP_URL} -central-plane-agent-key %s -central-plane-project-url ${SUPABASE_URL} -central-plane-publishable-key ${SUPABASE_PUBLISHABLE_KEY}

[Install]
WantedBy=multi-user.target
`,
		CompanyHostPrepareServiceName,
		CompanyHostPrepareServiceName,
		BlueclawServiceName,
		CompanyHostAgentKeyPath,
		companyHostCommonService(),
		CompanyPackageBinaryPath(AdmindName),
		CompanyHostAdmindListenAddress,
		CapabilitySocketPath,
		CompanyHostChatdEndpoint,
		BlueclawBaseURL,
		CompanyHostPolicyDocument,
		CompanyHostIdentitySeedPath,
		CompanyHostBuzzDatabasePath,
		CompanyHostBuzzRelayKeyPath,
		CompanyPackageBinaryPath(BuzzAdminName),
		BuzzRelayLocalURL,
		CompanyHostAccountLinksPath,
		CompanyPackageSkillsPath,
		CompanyHostAgentKeyPath)
}

func companyHostMaildUnit() string {
	return fmt.Sprintf(`[Unit]
Description=internkim Mail Daemon
After=network-online.target
Wants=network-online.target
ConditionPathExists=%s

[Service]
User=root
%sExecStart=%s -listen %s

[Install]
WantedBy=multi-user.target
`, CompanyHostEnvironmentPath, companyHostCommonService(), CompanyPackageBinaryPath(MaildName), CompanyHostMaildListenAddress)
}

func companyHostChatdUnit() string {
	return fmt.Sprintf(`[Unit]
Description=Buzz chatd bridge
After=network-online.target %s.service %s.service %s.service
Wants=network-online.target
ConditionPathExists=%s

[Service]
User=root
EnvironmentFile=%s
Environment=CHATD_BLUECLAW_BASE_URL=%s
Environment=CHATD_BUZZ_RELAY_URL=%s
Environment=CHATD_LISTEN_PORT=%s
Environment=CHATD_RELAY_INBOUND_URL=%s
Environment=CHATD_BUZZ_ACCOUNT_LINKS_PATH=%s
Environment=CHATD_ADMIND_BASE_URL=http://%s
%sExecStart=%s

[Install]
WantedBy=multi-user.target
`,
		BuzzRelayServiceName,
		AdmindServiceName,
		BlueclawServiceName,
		CompanyHostChatdSecretPath,
		CompanyHostChatdSecretPath,
		BlueclawBaseURL,
		BuzzRelayLocalURL,
		ChatdListenPort,
		CompanyHostArrivalsInboundURL,
		CompanyHostAccountLinksPath,
		CompanyHostAdmindListenAddress,
		companyHostCommonService(),
		CompanyPackageBinaryPath(ChatdName))
}

// CompanyHostSettingsFile is the shipped conffile. Every value in it is the default
// the unit would use anyway; it exists so an operator can change one without editing
// a unit dpkg owns, and so the object store has an address that is written down.
func CompanyHostSettingsFile() string {
	return strings.Join([]string{
		"# Settings for the company host installed by the internkim package.",
		"# dpkg keeps your edits across upgrades. Secrets do not belong here:",
		"# they live in " + CompanyHostSecretsRoot + ", which only root can read.",
		"",
		"# Where the messenger keeps attachments. Any S3-compatible server on this",
		"# box answers here; the access and secret keys live beside the other",
		"# secrets in " + CompanyHostRelayMediaPath + ".",
		"BUZZ_S3_ENDPOINT=http://" + BuzzMediaAddress,
		"BUZZ_S3_BUCKET=" + BuzzMediaBucket,
		"BUZZ_S3_REGION=us-east-1",
		"",
		"# The address clients reach this company's messenger at. Loopback until a",
		"# public host terminates TLS in front of it.",
		"RELAY_URL=" + BuzzRelayLocalURL,
		"",
	}, "\n")
}

// CompanyHostPrepareScript is the part of host/entrypoint.sh systemd does not take
// over: the runtime document, the roster the agent starts from, and staging the two
// private files the unprivileged blueclaw user has to read out of a directory only
// root can open. Everything after it in that script is a process systemd supervises.
//
// It is rendered here rather than kept as a file of its own so the paths it touches
// are the same constants the units name.
func CompanyHostPrepareScript() string {
	return fmt.Sprintf(`#!/bin/sh
set -e

: "${DATABASE_URL:?%[1]s names no DATABASE_URL}"
: "${MESSENGER_PLATFORM:?%[1]s names no MESSENGER_PLATFORM}"

[ -r %[2]s ] || { echo "no company identity at %[2]s; run internkim install" >&2; exit 1; }

install -d -o root -g %[3]s -m 0770 %[4]s
install -d -o root -g %[3]s -m 0750 %[5]s
install -o root -g %[3]s -m 0440 %[2]s %[6]s
[ ! -r %[7]s ] || install -o root -g %[3]s -m 0440 %[7]s %[8]s
install -d -o %[3]s -g %[3]s -m 0750 %[9]s
install -d -o %[3]s -g %[3]s -m 0750 %[10]s
install -d -o root -g root -m %[24]s %[11]s
install -d -o %[3]s -g %[3]s -m 0755 %[12]s
install -d -o %[3]s -g %[3]s -m 0755 %[12]s/.blueclaw

if [ -r %[13]s ]; then
  install -o root -g %[3]s -m 0640 %[13]s %[14]s
  echo "runtime document taken from %[13]s"
else
  CAPABILITY_SOCKET_PATH=%[15]s \
  BLUECLAW_BASE_URL=%[16]s \
  CHATD_ENDPOINT=%[17]s \
  WORKSPACE_ROOT_PATH=%[12]s \
  LOG_DIRECTORY_PATH=%[9]s \
  MODEL_API_KEY_PATH=%[8]s \
  ADMIN_ASSERTION_KEY_PATH=%[6]s \
  POSIX_HELPER_PATH=%[23]s \
    %[18]s --template %[19]s --capabilityd %[20]s --out %[14]s --work %[4]s
  chgrp %[3]s %[14]s
  chmod 0640 %[14]s
fi

if [ -r %[21]s ]; then
  install -o %[3]s -g %[3]s -m 0640 %[21]s %[22]s
else
  [ -s %[22]s ] || printf '{"people":[],"circles":[],"circleSync":{},"resourceAccess":[],"channels":[],"retention":{}}\n' > %[22]s
  chown %[3]s:%[3]s %[22]s
  chmod 0640 %[22]s
fi
`,
		CompanyHostEnvironmentPath,
		CompanyHostAgentKeyPath,
		BlueclawUser,
		CompanyHostRunPath,
		CompanyHostRunSecretsPath,
		CompanyHostRunAgentKeyPath,
		CompanyHostModelKeyPath,
		CompanyHostRunModelKeyPath,
		CompanyHostLogPath,
		CompanyHostBrowserStatePath,
		CompanyHostStateRoot,
		CompanyHostWorkspacePath,
		CompanyHostRuntimeOverridePath,
		CompanyHostRuntimeDocument,
		CapabilitySocketPath,
		BlueclawBaseURL,
		CompanyHostChatdEndpoint,
		CompanyPackageBinaryPath(RenderCompanyRuntimeName),
		CompanyPackageTemplatePath,
		CompanyPackageBinaryPath(CapabilitydName),
		CompanyHostPolicyOverridePath,
		CompanyHostPolicyDocument,
		CompanyHostPOSIXHelperPath,
		fmt.Sprintf("%04o", CompanyHostStateRootMode))
}
