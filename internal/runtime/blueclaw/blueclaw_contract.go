package blueclaw

import "path/filepath"

const (
	BlueclawName                       = "blueclaw"
	BlueclawServiceName                = "blueclaw"
	BlueclawUser                       = "blueclaw"
	BlueclawHomePath                   = "/home/blueclaw"
	BlueclawRootPath                   = "/root/.blueclaw"
	BlueclawWorkspacePath              = "/root/.blueclaw/workspace"
	BlueclawConfigPath                 = "/root/.blueclaw/config"
	BlueclawRuntimeConfigPath          = "/root/.blueclaw/config/runtime.json"
	BlueclawPolicyConfigPath           = "/root/.blueclaw/config/policy.json"
	BlueclawServicePath                = "/etc/systemd/system/blueclaw.service"
	BlueclawBinaryPath                 = "/usr/local/bin/blueclaw"
	BlueclawBaseURL                    = "http://127.0.0.1:8080"
	BlueclawHealthCheckPath            = "/admin/api/policy"
	BlueclawSubmodulePath              = ".dependency/blueclaw"
	BlueclawPolicyAdminID              = "00000000-0000-0000-0000-000000000001"
	BlueclawRuntimeLogLevel            = "debug"
	BlueclawSessionDirectory           = "/root/.blueclaw/workspace/sessions"
	BlueclawOpenRouterEnvFile          = "/root/.internkim/secrets/openrouter-api-key"
	BlueclawMattermostURLPath          = "/root/.internkim/env/mattermost-url"
	BlueclawMattermostTokenPath        = "/root/.internkim/env/bot-token"
	BlueclawSlackTokenPath             = "/root/.internkim/secrets/slack-bot-token"
	BlueclawDefaultModelName           = "google/gemini-3-flash-preview"
	BlueclawLiteRTLMWrapperPath        = "/usr/local/bin/blueclaw-litert-wrapper"
	BlueclawFirecrackerPath            = "/usr/local/bin/firecracker"
	BlueclawJailerPath                 = "/usr/local/bin/jailer"
	BlueclawKernelImagePath            = "/opt/blueclaw/vmlinux.bin"
	BlueclawRootFilesystemImagePath    = "/opt/blueclaw/rootfs.ext4"
	BlueclawWorkspaceImagePath         = "/var/lib/blueclaw/workspace.ext4"
	BlueclawSupervisorLogDirectoryPath = "/var/log/blueclaw-supervisor"
	BlueclawBridgeAuthorizedKeysPath   = "/var/lib/blueclaw/authorized_companions"
	BlueclawBridgeListenAddress        = "127.0.0.1:7778"
	BlueclawOpenRouterCompletionsURL   = "https://openrouter.ai/api/v1/chat/completions"
	BlueclawSlackAPIBaseURL            = "https://slack.com/api"
	BlueclawLiteRTLMModelRelativePath  = ".blueclaw/models/gemma-3n-E2B-it-int4.litertlm"
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
	"send-file",
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

func BlueclawLiteRTLMModelPath() string {
	return filepath.Join(BlueclawWorkspacePath, BlueclawLiteRTLMModelRelativePath)
}
