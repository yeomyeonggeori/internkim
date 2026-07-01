package blueclaw

import (
	"path/filepath"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

const (
	BlueclawName                          = "blueclaw"
	BlueclawSupervisorName                = "blueclaw-supervisor"
	BlueclawServiceName                   = "blueclaw"
	CapabilitydName                       = "internkim-capabilityd"
	AdmindName                            = "internkim-admind"
	LocalLLMRunnerName                    = "internkim-local-llm-runner"
	GraphitiMemorydName                   = "graphiti-memoryd"
	CapabilitydServiceName                = "internkim-capabilityd"
	AdmindServiceName                     = "internkim-admind"
	GraphitiMemorydServiceName            = "graphiti-memoryd"
	BlueclawUser                          = "blueclaw"
	BlueclawHomePath                      = "/home/blueclaw"
	BlueclawRootPath                      = "/root/.blueclaw"
	BlueclawWorkspacePath                 = "/root/.blueclaw/workspace"
	BlueclawConfigPath                    = "/root/.blueclaw/config"
	BlueclawMigrationPath                 = "/root/.blueclaw/workspace/.blueclaw/runtime/current/migrations"
	BlueclawRuntimeConfigPath             = "/root/.blueclaw/config/runtime.json"
	BlueclawPolicyConfigPath              = "/root/.blueclaw/config/policy.json"
	BlueclawServicePath                   = "/etc/systemd/system/blueclaw.service"
	CapabilitydServicePath                = "/etc/systemd/system/internkim-capabilityd.service"
	AdmindServicePath                     = "/etc/systemd/system/internkim-admind.service"
	GraphitiMemorydServicePath            = "/etc/systemd/system/graphiti-memoryd.service"
	BlueclawBinaryPath                    = "/usr/local/bin/blueclaw"
	BlueclawSupervisorBinaryPath          = "/usr/local/bin/blueclaw-supervisor"
	BlueclawPOSIXHelperPath               = "/usr/local/bin/blueclaw-posix-helper"
	CapabilitydBinaryPath                 = "/usr/local/bin/internkim-capabilityd"
	AdmindBinaryPath                      = "/usr/local/bin/internkim-admind"
	LocalLLMRunnerBinaryPath              = "/usr/local/bin/internkim-local-llm-runner"
	GraphitiMemorydPath                   = "/usr/local/bin/graphiti-memoryd"
	GraphitiMemorydPackagePath            = "/opt/internkim/graphiti_memoryd"
	GraphitiKuzuPath                      = "/root/.blueclaw/workspace/.blueclaw/graphiti/kuzu"
	GraphitiEndpoint                      = "http://127.0.0.1:7791"
	BlueclawBaseURL                       = "http://127.0.0.1:8080"
	AdmindBaseURL                         = "http://127.0.0.1:18080"
	BlueclawDatabaseName                  = "blueclaw"
	BlueclawGuestWorkspacePath            = "/workspace"
	BlueclawGuestConfigPath               = "/workspace/.blueclaw/config"
	BlueclawGuestMigrationPath            = "/workspace/.blueclaw/runtime/current/migrations"
	BlueclawGuestRuntimeConfigPath        = "/workspace/.blueclaw/config/runtime.json"
	BlueclawGuestPolicyConfigPath         = "/workspace/.blueclaw/config/policy.json"
	BlueclawGuestDatabaseSocketPath       = "/workspace/.blueclaw/postgres"
	BlueclawGuestDatabaseConnectionString = "user=blueclaw dbname=blueclaw host=/workspace/.blueclaw/postgres sslmode=disable"
	BlueclawHealthCheckPath               = "/admin/api/health"
	BlueclawSubmodulePath                 = ".dependency/blueclaw"
	BlueclawPolicyAdminID                 = "00000000-0000-0000-0000-000000000001"
	BlueclawRuntimeLogLevel               = "debug"
	BlueclawSessionDirectory              = "/root/.blueclaw/workspace/sessions"
	CapabilitySocketPath                  = "/run/internkim/capability.sock"
	CapabilityVSockHostCID                = 2
	CapabilityVSockPort                   = 7000
	BlueclawMattermostURLPath             = "/root/.internkim/env/mattermost-url"
	BlueclawMattermostTokenPath           = "/root/.internkim/secrets/mattermost-bot-token"
	BlueclawSlackTokenPath                = "/root/.internkim/secrets/slack-bot-token"
	BlueclawMattermostLocalURL            = "http://127.0.0.1:8065"
	LiteRTModelPath                       = "/root/.internkim/models/gemma-4-E4B-it.litertlm"
	LiteRTModelSourceURL                  = "https://huggingface.co/litert-community/gemma-4-E4B-it-litert-lm/resolve/main/gemma-4-E4B-it.litertlm"
	LiteRTModelRepository                 = "litert-community/gemma-4-E4B-it-litert-lm"
	LiteRTModelFilename                   = "gemma-4-E4B-it.litertlm"
	BlueclawDefaultModelName              = llmbackend.DefaultActionModelName
	BlueclawDefaultModelContextTokens     = 1048576
	BlueclawFirecrackerPath               = "/usr/local/bin/firecracker"
	BlueclawJailerPath                    = "/usr/local/bin/jailer"
	BlueclawRuntimeInstallPath            = "/opt/internkim/blueclaw-runtime"
	BlueclawRuntimeManifestPath           = "/opt/internkim/blueclaw-runtime/manifest.json"
	BlueclawPayloadManifestPath           = "/opt/internkim/blueclaw-runtime/payload-manifest.json"
	BlueclawKernelImagePath               = "/opt/internkim/blueclaw-runtime/vmlinux.bin"
	BlueclawRootFilesystemImagePath       = "/opt/internkim/blueclaw-runtime/rootfs.ext4"
	BlueclawWorkspaceImagePath            = "/var/lib/blueclaw/workspace.ext4"
	BlueclawRuntimeArtifactPath           = ".dependency/blueclaw-runtime"
	BlueclawPayloadArtifactPath           = ".dependency/blueclaw-payload"
	BlueclawGuestRuntimeCurrentPath       = "/workspace/.blueclaw/runtime/current"
	BlueclawGuestBinaryPath               = "/workspace/.blueclaw/runtime/current/bin/blueclaw"
	BlueclawSupervisorLogDirectoryPath    = "/var/log/blueclaw-supervisor"
	BlueclawBridgeAuthorizedKeysPath      = "/var/lib/blueclaw/authorized_companions"
	BlueclawBridgeListenAddress           = "127.0.0.1:7778"
	BlueclawSlackAPIBaseURL               = "https://slack.com/api"
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
	"uv",
	"python3",
	"curl",
	"jq",
	"capability",
	"download",
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
	"/workspace/.blueclaw/config",
	"/workspace/.blueclaw/postgres",
}

func BlueclawHealthCheckURL() string {
	return BlueclawBaseURL + BlueclawHealthCheckPath
}

func BlueclawHealthCheckCommand() string {
	return "curl --max-time 15 -fsS " + BlueclawHealthCheckURL() + " >/dev/null && echo ok || echo no"
}

func CapabilitydHealthCheckCommand() string {
	return "curl --max-time 5 -fsS --unix-socket " + CapabilitySocketPath + " http://internkim/health | jq -e '.status == \"ok\"' >/dev/null && echo ok || echo no"
}

func GraphitiMemorydHealthCheckCommand() string {
	return "curl --max-time 5 -fsS " + GraphitiEndpoint + "/health | jq -e '.status == \"ok\"' >/dev/null && echo ok || echo no"
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
