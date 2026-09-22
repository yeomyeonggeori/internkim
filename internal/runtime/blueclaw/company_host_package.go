package blueclaw

import (
	"fmt"
	"strconv"
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
	CompanyPackageName         = "internkim"
	CompanyPackageMaintainer   = "internkim <support@intern.kim>"
	CompanyPackageVendor       = "yeomyeonggeori"
	CompanyPackageHomepage     = "https://intern.kim"
	CompanyPackageSection      = "admin"
	CompanyPackageBinaryRoot   = "/usr/bin"
	CompanyPackageLibraryRoot  = "/opt/internkim"
	CompanyPackageHelperRoot   = "/usr/lib/internkim"
	CompanyPackageUnitRoot     = "/usr/lib/systemd/system"
	CompanyPackageSkillsPath   = "/opt/internkim/skills"
	CompanyPackageTemplatePath = "/opt/internkim/runtime.template.json"

	// The interpreter the document skills run under. Its site-packages are
	// resolved once, when the package is built, so no customer machine resolves
	// a wheel. capabilityd hands this path to the file reader, the image builds
	// the same one, and the .deb carries it.
	CompanyPackageDocumentVenvPath   = "/opt/internkim/document-venv"
	CompanyPackageDocumentPythonPath = CompanyPackageDocumentVenvPath + "/bin/python"
	CompanyPackageMigrationPath      = "/opt/blueclaw/migrations"
	CompanyPackagePreparePath        = "/usr/lib/internkim/prepare-company-host"

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
	return CompanyHostSystemdUnits(DebianCompanyHostLayout())
}

// CompanyHostSystemdUnits renders the bundle for systemd. What each unit runs comes
// from CompanyHostServices; what only systemd can say about it — the ordering, the
// condition, the Documentation link — is here, because launchd has no counterpart for
// any of it.
func CompanyHostSystemdUnits(layout CompanyHostLayout) []CompanyPackageUnit {
	units := []CompanyPackageUnit{}
	for _, service := range CompanyHostServices(layout) {
		if service.Name == RelayServiceName {
			units = append(units, CompanyPackageUnit{
				Name:     service.Name,
				Contents: relayServiceUnit(layout.BinaryPath(RelayName), CompanyHostRelayStateDirectoryName),
			})
			continue
		}
		units = append(units, CompanyPackageUnit{Name: service.Name, Contents: systemdUnitFor(service)})
	}
	return units
}

// companyHostUnitOrdering is systemd's half of the bundle and systemd's alone.
// launchd.plist(5): "Unlike many bootstrapping daemons, launchd has no explicit
// dependency model. Interdependencies are expected to be solved through the use of
// IPC." So none of this crosses to the Mac, and what stands in its place there is the
// readiness polling internal/companyhost already does on both.
type companyHostUnitOrdering struct {
	After            []string
	Wants            []string
	Requires         []string
	BindsTo          []string
	DocumentationURL string
}

func companyHostOrderingFor(serviceName string) companyHostUnitOrdering {
	switch serviceName {
	case CompanyHostPrepareServiceName:
		return companyHostUnitOrdering{
			DocumentationURL: "https://github.com/yeomyeonggeori/internkim/blob/main/docs/internal/native-packaging.md",
		}
	case BuzzMediaServiceName:
		return companyHostUnitOrdering{}
	case BuzzRelayServiceName:
		return companyHostUnitOrdering{
			After:   []string{"postgresql.service", "redis-server.service", BuzzMediaServiceName + ".service"},
			Wants:   []string{"redis-server.service", BuzzMediaServiceName + ".service"},
			BindsTo: []string{"postgresql.service"},
		}
	case CapabilitydServiceName:
		return companyHostUnitOrdering{
			After:    []string{CompanyHostPrepareServiceName + ".service"},
			Requires: []string{CompanyHostPrepareServiceName + ".service"},
		}
	case BlueclawServiceName:
		return companyHostUnitOrdering{
			After:    []string{"postgresql.service", CompanyHostPrepareServiceName + ".service", CapabilitydServiceName + ".service"},
			Wants:    []string{CapabilitydServiceName + ".service"},
			Requires: []string{CompanyHostPrepareServiceName + ".service"},
			BindsTo:  []string{"postgresql.service"},
		}
	case AdmindServiceName:
		return companyHostUnitOrdering{
			After:    []string{CompanyHostPrepareServiceName + ".service"},
			Wants:    []string{BlueclawServiceName + ".service"},
			Requires: []string{CompanyHostPrepareServiceName + ".service"},
		}
	case ChatdServiceName:
		return companyHostUnitOrdering{
			After: []string{BuzzRelayServiceName + ".service", AdmindServiceName + ".service", BlueclawServiceName + ".service"},
		}
	}
	return companyHostUnitOrdering{}
}

func systemdUnitFor(service CompanyHostService) string {
	ordering := companyHostOrderingFor(service.Name)
	unit := &strings.Builder{}
	unit.WriteString("[Unit]\nDescription=" + service.Description + "\n")
	if ordering.DocumentationURL != "" {
		unit.WriteString("Documentation=" + ordering.DocumentationURL + "\n")
	}
	unit.WriteString("After=" + strings.Join(append([]string{"network-online.target"}, ordering.After...), " ") + "\n")
	unit.WriteString("Wants=" + strings.Join(append([]string{"network-online.target"}, ordering.Wants...), " ") + "\n")
	writeSystemdList(unit, "Requires", ordering.Requires)
	writeSystemdList(unit, "BindsTo", ordering.BindsTo)
	unit.WriteString("ConditionPathExists=" + service.WaitsForTheFileAtPath + "\n")

	unit.WriteString("\n[Service]\n")
	if service.RunsOnceAndStays {
		unit.WriteString("Type=oneshot\nRemainAfterExit=yes\n")
	}
	if service.Account == "" {
		unit.WriteString("User=root\n")
	} else {
		unit.WriteString("User=" + service.Account + "\nGroup=" + service.Account + "\n")
	}
	if service.WorkingDirectory != "" {
		unit.WriteString("WorkingDirectory=" + service.WorkingDirectory + "\n")
	}
	for _, source := range service.Environment {
		writeSystemdEnvironment(unit, source)
	}
	if service.RestartAfterSeconds > 0 {
		unit.WriteString(systemdRestartSetting(service) + "\nRestartSec=" + strconv.Itoa(service.RestartAfterSeconds) + "\n")
	}
	unit.WriteString("ExecStart=" + service.CommandLine() + "\n")
	if service.StopTimeoutSeconds > 0 {
		unit.WriteString("KillMode=mixed\nTimeoutStopSec=" + strconv.Itoa(service.StopTimeoutSeconds) + "\n")
	}
	unit.WriteString("\n[Install]\nWantedBy=multi-user.target\n")
	return unit.String()
}

func systemdRestartSetting(service CompanyHostService) string {
	if service.RestartsEvenOnACleanExit {
		return "Restart=always"
	}
	return "Restart=on-failure"
}

func writeSystemdList(unit *strings.Builder, name string, values []string) {
	if len(values) == 0 {
		return
	}
	unit.WriteString(name + "=" + strings.Join(values, " ") + "\n")
}

func writeSystemdEnvironment(unit *strings.Builder, source CompanyHostEnvironmentSource) {
	if source.FilePath != "" {
		prefix := ""
		if source.IsOptional {
			prefix = "-"
		}
		unit.WriteString("EnvironmentFile=" + prefix + source.FilePath + "\n")
		return
	}
	for _, value := range source.Settings {
		unit.WriteString("Environment=" + value.Name + "=" + value.Value + "\n")
	}
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
	return CompanyHostPrepareScriptFor(DebianCompanyHostLayout())
}

// CompanyHostPrepareScriptFor renders it for one machine's layout. Every program
// it runs — install, chgrp, chmod, printf, test — is in POSIX and in BSD
// userland, so the script itself crosses; only the paths move.
func CompanyHostPrepareScriptFor(layout CompanyHostLayout) string {
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
		layout.RunPath,
		layout.RunSecretsPath(),
		layout.RunAgentKeyPath(),
		CompanyHostModelKeyPath,
		layout.RunModelKeyPath(),
		CompanyHostLogPath,
		CompanyHostBrowserStatePath,
		CompanyHostStateRoot,
		layout.WorkspacePath,
		CompanyHostRuntimeOverridePath,
		layout.RuntimeDocumentPath(),
		layout.CapabilitySocketPath(),
		BlueclawBaseURL,
		CompanyHostChatdEndpoint,
		layout.BinaryPath(RenderCompanyRuntimeName),
		layout.RuntimeTemplatePath(),
		layout.BinaryPath(CapabilitydName),
		CompanyHostPolicyOverridePath,
		layout.PolicyDocumentPath(),
		layout.POSIXHelperPath(),
		fmt.Sprintf("%04o", CompanyHostStateRootMode))
}
