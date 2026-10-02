package blueclaw

import (
	"strconv"
	"strings"
)

// What the company host runs, said once, for the two supervisors that run it.
//
// systemd and launchd agree on less than they disagree on, and the line between
// them is what this file is for. A fact both can express — the program and its
// arguments, the account, the working directory, the environment, whether a
// crash is restarted and how long a stop may take — is declared here and
// rendered twice. A fact only systemd can express — After=, Requires=,
// BindsTo=, ConditionPathExists= — stays in the systemd template, because
// launchd.plist(5) says outright that "launchd has no explicit dependency
// model" and a launchd emitter pretending otherwise would render a promise the
// supervisor does not keep. What stands where that ordering stood is the
// readiness polling in internal/companyhost.
//
// Two differences are not a matter of taste. EnvironmentFile= has no launchd
// counterpart at all: launchd bakes EnvironmentVariables into the plist when it
// is written, which is why the plists are written by `internkim install` after
// the company exists rather than shipped with the package. And systemd expands
// ${NAME} inside ExecStart while launchd passes ProgramArguments to execve
// untouched, so the launchd emitter resolves those references against the same
// merged environment and refuses a name nothing sets.

// CompanyHostService is one process of the bundle, in the terms both supervisors
// share.
type CompanyHostService struct {
	Name        string
	Description string
	// Command is the program and its arguments, already split the way execve
	// takes them. A ${NAME} in an argument is resolved from Environment.
	Command []string
	// Account is the user the process runs as. Empty means root.
	Account string
	// WorkingDirectory is empty for every service that does not care.
	WorkingDirectory string
	// Environment is read in order, so a later source wins over an earlier one.
	// The operator's settings file is last in every service that reads it.
	Environment []CompanyHostEnvironmentSource
	// RunsOnceAndStays is a step rather than a daemon: systemd's Type=oneshot
	// with RemainAfterExit, launchd's LaunchOnlyOnce.
	RunsOnceAndStays bool
	// RestartAfterSeconds is how long the supervisor waits before starting a
	// crashed process again. Zero means it is not restarted.
	RestartAfterSeconds int
	// RestartsEvenOnACleanExit is the relay, which is the one service that must
	// be up whether or not anything else is.
	RestartsEvenOnACleanExit bool
	// StopTimeoutSeconds is how long a stop may take before the supervisor
	// kills the process. Zero takes the supervisor's own default.
	StopTimeoutSeconds int
	// WaitsForTheFileAtPath is what systemd puts in ConditionPathExists, so a
	// box with the package and no company sits idle. launchd has no equivalent
	// it reads the way it is documented, so this is systemd's alone and the
	// plists are only written once the file is there.
	WaitsForTheFileAtPath string
}

// CompanyHostEnvironmentSource is one place a service's environment comes from,
// in the order it is read.
type CompanyHostEnvironmentSource struct {
	// FilePath is a file of NAME=VALUE lines. Empty means Settings below.
	FilePath string
	// IsOptional says a missing file is not a failure.
	IsOptional bool
	Settings   []EnvironmentSetting
}

// EnvironmentSetting is one NAME=VALUE the unit itself carries.
type EnvironmentSetting struct {
	Name  string
	Value string
}

func environmentFile(path string) CompanyHostEnvironmentSource {
	return CompanyHostEnvironmentSource{FilePath: path}
}

func optionalEnvironmentFile(path string) CompanyHostEnvironmentSource {
	return CompanyHostEnvironmentSource{FilePath: path, IsOptional: true}
}

func environmentSettings(settings ...EnvironmentSetting) CompanyHostEnvironmentSource {
	return CompanyHostEnvironmentSource{Settings: settings}
}

// setting keeps the declarations below readable; every value is a constant from
// this package.
func setting(name string, value string) EnvironmentSetting {
	return EnvironmentSetting{Name: name, Value: value}
}

// companyHostCommonEnvironment is what every unit of the bundle shares: the
// company's own environment, then the operator's file last so an edited setting
// wins over a rendered default.
func companyHostCommonEnvironment() []CompanyHostEnvironmentSource {
	return []CompanyHostEnvironmentSource{
		environmentFile(CompanyHostEnvironmentPath),
		optionalEnvironmentFile(CompanyHostSettingsPath),
	}
}

// The shared file first, then what this service alone needs, then the
// operator's. systemd applies these in the order they are written, so a shared
// file read last silently overrides a service's own: the messenger and the
// agent both read DATABASE_URL and they are not the same database, which put
// the messenger's whole store in the agent's one and left every buzz-admin
// call reading an empty database.
func withCommonEnvironment(sources ...CompanyHostEnvironmentSource) []CompanyHostEnvironmentSource {
	composed := []CompanyHostEnvironmentSource{environmentFile(CompanyHostEnvironmentPath)}
	composed = append(composed, sources...)
	return append(composed, optionalEnvironmentFile(CompanyHostSettingsPath))
}

// CompanyHostServices is the bundle, in start order.
func CompanyHostServices(layout CompanyHostLayout) []CompanyHostService {
	return []CompanyHostService{
		companyHostRelayService(layout),
		companyHostPrepareService(layout),
		companyHostMediaService(layout),
		companyHostMessengerService(layout),
		companyHostCapabilityService(layout),
		companyHostAgentService(layout),
		companyHostAdminService(layout),
		companyHostMailService(layout),
		companyHostMessengerBridgeService(layout),
	}
}

// CompanyHostServiceNamed is how an emitter reaches one service without
// rebuilding the list around it.
func CompanyHostServiceNamed(layout CompanyHostLayout, name string) (CompanyHostService, bool) {
	for _, service := range CompanyHostServices(layout) {
		if service.Name == name {
			return service, true
		}
	}
	return CompanyHostService{}, false
}

func companyHostRelayService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        RelayServiceName,
		Description: "internkim relay",
		Command:     []string{layout.BinaryPath(RelayName)},
		Account:     RelayUserName,
		Environment: []CompanyHostEnvironmentSource{
			environmentFile(RelayEnvironmentFilePath),
			environmentSettings(companyHostRelaySettings(layout)...),
		},
		RestartAfterSeconds:      30,
		RestartsEvenOnACleanExit: true,
		WaitsForTheFileAtPath:    RelayEnvironmentFilePath,
	}
}

func companyHostRelaySettings(layout CompanyHostLayout) []EnvironmentSetting {
	return []EnvironmentSetting{
		setting("RELAY_STATE_DIR", RelayStateDirectoryPath(CompanyHostRelayStateDirectoryName)),
		setting("MESSENGER_PLATFORM", BlueclawMessengerPlatform),
		setting("RUST_LOG", "buzz_relay=info,buzz_media=info"),
		setting("AGENT_API_KEY_PATH", RelayAgentKeyPath),
		setting("CHATD_BASE_URL", CompanyHostChatdEndpoint),
		setting("MESSENGER_RELAY_URL", BuzzRelayLocalURL),
		setting("ADMIND_BASE_URL", "http://"+CompanyHostAdmindListenAddress),
		setting("ADMIND_SOCKET_PATH", layout.AdmindSocketPath()),
		setting("BLUECLAW_ACP_SOCKET_PATH", layout.ACPSocketPath()),
		setting("WORKSPACE_ROOT_PATH", layout.WorkspacePath),
	}
}

// systemd.exec(5): "Settings from these files override settings made with
// Environment=", whatever order the two are written in.
func CompanyHostRelaySettingNames() []string {
	names := []string{}
	for _, value := range companyHostRelaySettings(LinuxCompanyHostLayout()) {
		names = append(names, value.Name)
	}
	return names
}

func companyHostPrepareService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:                  CompanyHostPrepareServiceName,
		Description:           "internkim company host preparation",
		Command:               []string{layout.PrepareScriptPath()},
		Environment:           companyHostCommonEnvironment(),
		RunsOnceAndStays:      true,
		WaitsForTheFileAtPath: CompanyHostAgentKeyPath,
	}
}

// The posix backend serves every directory under its root as a bucket, so the
// bucket is a directory the install creates rather than something an S3 client
// makes. --versioning-dir is absent for the reason BuzzMediaServiceUnit gives.
func companyHostMediaService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        BuzzMediaServiceName,
		Description: "Buzz Media Store",
		Command: []string{
			layout.BinaryPath(BuzzMediaProgramName),
			"--port", BuzzMediaAddress,
			"--health", BuzzMediaHealthPath,
			"posix", CompanyHostMediaRootPath,
		},
		Environment:           withCommonEnvironment(environmentFile(CompanyHostMediaSecretPath)),
		RestartAfterSeconds:   5,
		WaitsForTheFileAtPath: CompanyHostMediaSecretPath,
	}
}

func companyHostMessengerService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        BuzzRelayServiceName,
		Description: "Buzz Relay",
		Command:     []string{layout.BinaryPath(BuzzRelayName)},
		Environment: withCommonEnvironment(
			environmentFile(CompanyHostBuzzRelayKeyPath),
			environmentFile(CompanyHostBuzzDatabasePath),
			optionalEnvironmentFile(CompanyHostRelayMediaPath),
			environmentSettings(
				setting("BUZZ_BIND_ADDR", BuzzRelayBindAddress),
				setting("BUZZ_HEALTH_PORT", BuzzRelayHealthPort),
				setting("REDIS_URL", layout.CacheURL()),
				setting("BUZZ_AUTO_MIGRATE", "1"),
				setting("BUZZ_REQUIRE_RELAY_MEMBERSHIP", "true"),
				setting("BUZZ_DB_POOL_SIZE", strconv.Itoa(MessengerDatabaseConnections)),
			),
		),
		RestartAfterSeconds:   5,
		StopTimeoutSeconds:    30,
		WaitsForTheFileAtPath: CompanyHostBuzzRelayKeyPath,
	}
}

func companyHostCapabilityService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        CapabilitydServiceName,
		Description: "internkim Capability Daemon",
		Command: []string{
			layout.BinaryPath(CapabilitydName),
			"--socket", layout.CapabilitySocketPath(),
			"--openrouter-key", CompanyHostModelKeyPath,
			"--local-inference-mode", "remote",
			"--blueclaw-url", BlueclawBaseURL,
			"--blueclaw-workspace", layout.WorkspacePath,
			"--admind-url", "http://" + CompanyHostAdmindListenAddress,
			"--admind-socket", layout.AdmindSocketPath(),
			"--chatd-endpoint", CompanyHostChatdEndpoint,
			"--chatd-platform", "${MESSENGER_PLATFORM}",
			"--device-browser", layout.BinaryPath(DeviceBrowserName),
			"--device-browser-state-dir", CompanyHostBrowserStatePath,
			"--device-browser-first-port", CompanyHostBrowserFirstPort,
			"--device-browser-capacity", CompanyHostBrowserCapacity,
			"--device-browser-user", BlueclawUser,
			"--file-read-python", layout.DocumentPythonPath(),
		},
		Environment:           companyHostCommonEnvironment(),
		RestartAfterSeconds:   5,
		WaitsForTheFileAtPath: CompanyHostAgentKeyPath,
	}
}

func companyHostAgentService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        BlueclawServiceName,
		Description: "Blueclaw",
		Command: []string{
			layout.BinaryPath(BlueclawName),
			"-runtime", layout.RuntimeDocumentPath(),
			"-policy", layout.PolicyDocumentPath(),
			"-acp-socket", layout.ACPSocketPath(),
			"-inbound", "acp",
		},
		Account:          BlueclawUser,
		WorkingDirectory: layout.WorkspacePath,
		Environment: withCommonEnvironment(environmentSettings(
			setting("HOME", layout.AgentHomePath),
			setting("BLUECLAW_BUNDLED_SKILLS_PATH", layout.SkillsPath()),
		)),
		RestartAfterSeconds:   5,
		StopTimeoutSeconds:    30,
		WaitsForTheFileAtPath: layout.RuntimeDocumentPath(),
	}
}

func companyHostAdminService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        AdmindServiceName,
		Description: "internkim Admin Gateway",
		Command: []string{
			layout.BinaryPath(AdmindName),
			"-listen", CompanyHostAdmindListenAddress,
			"-listen-socket", layout.AdmindSocketPath(),
			"-capability-socket", layout.CapabilitySocketPath(),
			"-chatd-endpoint", CompanyHostChatdEndpoint,
			"-chatd-platform", "${MESSENGER_PLATFORM}",
			"-blueclaw-url", BlueclawBaseURL,
			"-blueclaw-policy", layout.PolicyDocumentPath(),
			"-blueclaw-workspace", layout.WorkspacePath,
			"-buzz-key-seed-path", CompanyHostIdentitySeedPath,
			"-buzz-database-url-path", CompanyHostBuzzDatabasePath,
			"-buzz-relay-key-path", CompanyHostBuzzRelayKeyPath,
			"-buzz-admin-command", layout.BinaryPath(BuzzAdminName),
			"-buzz-relay-url", BuzzRelayLocalURL,
			"-buzz-relay-public-url", "${RELAY_URL}",
			"-buzz-account-links", CompanyHostAccountLinksPath,
			"-central-plane-app-url", "${INTERNKIM_APP_URL}",
			"-central-plane-agent-key", CompanyHostAgentKeyPath,
			"-blueclaw-assertion-key", CompanyHostAssertionKeyPath,
			"-central-plane-project-url", "${SUPABASE_URL}",
			"-central-plane-publishable-key", "${SUPABASE_PUBLISHABLE_KEY}",
		},
		Environment:           companyHostCommonEnvironment(),
		RestartAfterSeconds:   5,
		WaitsForTheFileAtPath: CompanyHostAgentKeyPath,
	}
}

func companyHostMailService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:                  CompanyHostMaildServiceName,
		Description:           "internkim Mail Daemon",
		Command:               []string{layout.BinaryPath(MaildName), "-listen", CompanyHostMaildListenAddress},
		Environment:           companyHostCommonEnvironment(),
		RestartAfterSeconds:   5,
		WaitsForTheFileAtPath: CompanyHostEnvironmentPath,
	}
}

func companyHostMessengerBridgeService(layout CompanyHostLayout) CompanyHostService {
	return CompanyHostService{
		Name:        ChatdServiceName,
		Description: "Buzz chatd bridge",
		Command:     []string{layout.BinaryPath(ChatdName)},
		Environment: withCommonEnvironment(
			environmentFile(CompanyHostChatdSecretPath),
			environmentSettings(
				setting("CHATD_BLUECLAW_BASE_URL", BlueclawBaseURL),
				setting("CHATD_BUZZ_RELAY_DIAL_URL", BuzzRelayLocalURL),
				setting("CHATD_LISTEN_PORT", ChatdListenPort),
				setting("CHATD_RELAY_INBOUND_URL", CompanyHostArrivalsInboundURL),
				setting("CHATD_BUZZ_ACCOUNT_LINKS_PATH", CompanyHostAccountLinksPath),
				setting("CHATD_ADMIND_BASE_URL", "http://"+CompanyHostAdmindListenAddress),
				setting("CHATD_STATE_DIRECTORY", CompanyHostChatdStatePath),
			),
		),
		RestartAfterSeconds:   5,
		WaitsForTheFileAtPath: CompanyHostChatdSecretPath,
	}
}

// CommandLine is the command as one line, for a supervisor that takes a string.
func (service CompanyHostService) CommandLine() string {
	return strings.Join(service.Command, " ")
}
