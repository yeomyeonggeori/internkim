package blueclaw

import "path/filepath"

const (
	BlueclawName                       = "blueclaw"
	BlueclawServiceName                = "blueclaw"
	CapabilitydName                    = "internkim-capabilityd"
	AdmindName                         = "internkim-admind"
	LiteRTWrapperName                  = "internkim-litert-wrapper"
	GraphitiMemorydName                = "graphiti-memoryd"
	CapabilitydServiceName             = "internkim-capabilityd"
	AdmindServiceName                  = "internkim-admind"
	GraphitiMemorydServiceName         = "graphiti-memoryd"
	BlueclawUser                       = "blueclaw"
	BlueclawHomePath                   = "/home/blueclaw"
	BlueclawRootPath                   = "/root/.blueclaw"
	BlueclawWorkspacePath              = "/root/.blueclaw/workspace"
	BlueclawConfigPath                 = "/root/.blueclaw/config"
	BlueclawMigrationPath              = "/root/.blueclaw/migrations"
	BlueclawRuntimeConfigPath          = "/root/.blueclaw/config/runtime.json"
	BlueclawPolicyConfigPath           = "/root/.blueclaw/config/policy.json"
	BlueclawServicePath                = "/etc/systemd/system/blueclaw.service"
	CapabilitydServicePath             = "/etc/systemd/system/internkim-capabilityd.service"
	AdmindServicePath                  = "/etc/systemd/system/internkim-admind.service"
	GraphitiMemorydServicePath         = "/etc/systemd/system/graphiti-memoryd.service"
	BlueclawBinaryPath                 = "/usr/local/bin/blueclaw"
	CapabilitydBinaryPath              = "/usr/local/bin/internkim-capabilityd"
	AdmindBinaryPath                   = "/usr/local/bin/internkim-admind"
	LiteRTWrapperBinaryPath            = "/usr/local/bin/internkim-litert-wrapper"
	GraphitiMemorydPath                = "/usr/local/bin/graphiti-memoryd"
	GraphitiMemorydPackagePath         = "/opt/internkim/graphiti_memoryd"
	GraphitiKuzuPath                   = "/root/.blueclaw/workspace/.blueclaw/graphiti/kuzu"
	GraphitiEndpoint                   = "http://127.0.0.1:7791"
	BlueclawBaseURL                    = "http://127.0.0.1:8080"
	AdmindBaseURL                      = "http://127.0.0.1:18080"
	BlueclawDatabaseName               = "blueclaw"
	BlueclawDatabaseConnectionString   = "postgres://blueclaw@/blueclaw?host=/var/run/postgresql&sslmode=disable"
	BlueclawHealthCheckPath            = "/admin/api/policy"
	BlueclawSubmodulePath              = ".dependency/blueclaw"
	BlueclawPolicyAdminID              = "00000000-0000-0000-0000-000000000001"
	BlueclawRuntimeLogLevel            = "debug"
	BlueclawSessionDirectory           = "/root/.blueclaw/workspace/sessions"
	CapabilitySocketPath               = "/run/internkim/capability.sock"
	BlueclawMattermostURLPath          = "/root/.internkim/env/mattermost-url"
	BlueclawMattermostTokenPath        = "/root/.internkim/secrets/mattermost-bot-token"
	BlueclawSlackTokenPath             = "/root/.internkim/secrets/slack-bot-token"
	LiteRTModelPath                    = "/root/.internkim/models/gemma-4-E4B-it.litertlm"
	LiteRTModelSourceURL               = "https://huggingface.co/litert-community/gemma-4-E4B-it-litert-lm/resolve/main/gemma-4-E4B-it.litertlm"
	LiteRTModelRepository              = "litert-community/gemma-4-E4B-it-litert-lm"
	LiteRTModelFilename                = "gemma-4-E4B-it.litertlm"
	BlueclawDefaultModelName           = "google/gemini-3.1-flash-lite-preview"
	BlueclawDefaultModelContextTokens  = 1048576
	BlueclawFirecrackerPath            = "/usr/local/bin/firecracker"
	BlueclawJailerPath                 = "/usr/local/bin/jailer"
	BlueclawKernelImagePath            = "/opt/blueclaw/vmlinux.bin"
	BlueclawRootFilesystemImagePath    = "/opt/blueclaw/rootfs.ext4"
	BlueclawWorkspaceImagePath         = "/var/lib/blueclaw/workspace.ext4"
	BlueclawSupervisorLogDirectoryPath = "/var/log/blueclaw-supervisor"
	BlueclawBridgeAuthorizedKeysPath   = "/var/lib/blueclaw/authorized_companions"
	BlueclawBridgeListenAddress        = "127.0.0.1:7778"
	BlueclawSlackAPIBaseURL            = "https://slack.com/api"
)

var BlueclawAllowedExecutables = []string{
	"bash",
	"sh",
	"git",
	"rg",
	"find",
	"ls",
	"cat",
	"sed",
	"awk",
	"grep",
	"mkdir",
	"touch",
	"cp",
	"mv",
	"rm",
	"tar",
	"gzip",
	"bun",
	"marp",
	"python3",
	"curl",
	"jq",
	"download",
	"gws",
	"gws-bot",
}

var BlueclawDeniedExecutables = []string{
	"sudo",
	"su",
	"doas",
	"pkexec",
	"mount",
	"umount",
	"systemctl",
	"service",
	"apt",
	"apt-get",
	"dpkg",
	"yum",
	"dnf",
	"apk",
	"pacman",
	"snap",
	"flatpak",
	"rpm",
	"modprobe",
	"insmod",
	"shutdown",
	"reboot",
	"halt",
	"poweroff",
	"useradd",
	"userdel",
	"usermod",
	"groupadd",
	"groupdel",
	"passwd",
	"chpasswd",
	"visudo",
	"iptables",
	"nft",
	"sysctl",
}

var BlueclawDeniedPathPrefixes = []string{
	"/etc",
	"/boot",
	"/sys",
	"/proc",
	"/dev",
	"/run",
	"/var/lib",
	"/var/run",
	"/usr",
	"/opt",
	"/srv",
}

func BlueclawHealthCheckURL() string {
	return BlueclawBaseURL + BlueclawHealthCheckPath
}

func BlueclawHealthCheckCommand() string {
	return "curl -fsS " + BlueclawHealthCheckURL() + " >/dev/null && echo ok || echo no"
}

func BlueclawWorkspaceSkillPath(skillName string) string {
	return filepath.Join(BlueclawWorkspacePath, "skills", skillName)
}

func BlueclawWorkspaceBinaryPath(binaryName string) string {
	return filepath.Join(BlueclawWorkspacePath, "bin", binaryName)
}

func BlueclawSubmoduleRoot(scriptDir string) string {
	return filepath.Join(scriptDir, BlueclawSubmodulePath)
}
