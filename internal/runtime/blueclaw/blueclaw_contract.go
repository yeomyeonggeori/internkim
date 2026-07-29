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
	LLMDName                              = "blueclaw-llmd"
	LLMDServiceName                       = "blueclaw-llmd"
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
	LLMDServicePath                       = "/etc/systemd/system/blueclaw-llmd.service"
	BlueclawBinaryPath                    = "/usr/local/bin/blueclaw"
	BlueclawSupervisorBinaryPath          = "/usr/local/bin/blueclaw-supervisor"
	BlueclawPOSIXHelperPath               = "/usr/local/bin/blueclaw-posix-helper"
	CapabilitydBinaryPath                 = "/usr/local/bin/internkim-capabilityd"
	AdmindBinaryPath                      = "/usr/local/bin/internkim-admind"
	LocalLLMRunnerBinaryPath              = "/usr/local/bin/internkim-local-llm-runner"
	GraphitiMemorydPath                   = "/usr/local/bin/graphiti-memoryd"
	GraphitiMemorydPackagePath            = "/opt/internkim/graphiti_memoryd"
	LLMDBinaryPath                        = "/usr/local/bin/blueclaw-llmd"
	LLMDRuntimeDirectoryPath              = "/run/blueclaw-llmd"
	LLMDSocketPath                        = LLMDRuntimeDirectoryPath + "/llmd.sock"
	LLMDRuntimeAuthKeyPath                = LLMDRuntimeDirectoryPath + "/llmd-auth-key"
	LLMDRuntimeOpenRouterKeyPath          = LLMDRuntimeDirectoryPath + "/openrouter-api-key"
	LLMDAuthKeyPath                       = "/root/.internkim/secrets/llmd-auth-key"
	LLMDServiceCredentialDirectoryPath    = "/var/lib/internkim/llmd-credentials"
	LLMDServiceAuthKeyPath                = "/var/lib/internkim/llmd-credentials/llmd-auth-key"
	LLMDServiceOpenRouterKeyPath          = "/var/lib/internkim/llmd-credentials/openrouter-api-key"
	OpenRouterKeyPath                     = "/root/.internkim/secrets/openrouter-api-key"
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
	BuzzRelayName                         = "buzz-relay"
	BuzzAdminName                         = "buzz-admin"
	BuzzRelayServiceName                  = "buzz-relay"
	BuzzRelayServicePath                  = "/etc/systemd/system/buzz-relay.service"
	BuzzRelayBinaryPath                   = "/usr/local/bin/buzz-relay"
	BuzzAdminBinaryPath                   = "/usr/local/bin/buzz-admin"
	BuzzRelayDatabaseName                 = "buzz"
	BuzzRelayDatabaseUser                 = "buzz"
	BuzzRelayDatabasePasswordPath         = "/root/.internkim/secrets/buzz-db-pass"
	BuzzRelayKeyEnvironmentFilePath       = "/root/.internkim/secrets/buzz-relay-env"
	BuzzRelayDatabaseEnvironmentFilePath  = "/root/.internkim/secrets/buzz-relay-db"
	BuzzRelayBindAddress                  = "127.0.0.1:3000"
	BuzzRelayHealthPort                   = "3001"
	BuzzRelayLocalURL                     = "ws://127.0.0.1:3000"
	BuzzRelayRedisURL                     = "redis://127.0.0.1:6379"
	BuzzRelayArtifactPath                 = ".dependency/buzz-relay"
	MinioServiceName                      = "buzz-minio"
	MinioServicePath                      = "/etc/systemd/system/buzz-minio.service"
	MinioBinaryPath                       = "/usr/local/bin/minio"
	McBinaryPath                          = "/usr/local/bin/mc"
	MinioBinaryURL                        = "https://dl.min.io/server/minio/release/linux-arm64/minio"
	McBinaryURL                           = "https://dl.min.io/client/mc/release/linux-arm64/mc"
	MinioDataPath                         = "/var/lib/buzz-minio"
	MinioAddress                          = "127.0.0.1:9000"
	MinioConsoleAddress                   = "127.0.0.1:9001"
	MinioEnvironmentFilePath              = "/root/.internkim/secrets/buzz-minio-env"
	MinioPasswordPath                     = "/root/.internkim/secrets/buzz-minio-pass"
	BuzzMediaBucket                       = "buzz-media"
	BuzzMediaAccessKey                    = "buzzrelay"
	BuzzRelayS3EnvironmentFilePath        = "/root/.internkim/secrets/buzz-relay-s3"
)

func BlueclawHealthCheckURL() string {
	return BlueclawBaseURL + BlueclawHealthCheckPath
}

func BlueclawHealthCheckCommand() string {
	return "curl --max-time 15 -fsS " + BlueclawHealthCheckURL() + " >/dev/null && echo ok || echo no"
}

func CapabilitydHealthCheckCommand() string {
	return "curl --max-time 5 -fsS --unix-socket " + CapabilitySocketPath + " http://internkim/health | jq -e '.status == \"ok\"' >/dev/null && echo ok || echo no"
}

func LLMDHealthCheckCommand() string {
	return "curl --max-time 5 -fsS --unix-socket " + LLMDSocketPath + " http://blueclaw-llmd/health | jq -e '.status == \"ok\"' >/dev/null && echo ok || echo no"
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
