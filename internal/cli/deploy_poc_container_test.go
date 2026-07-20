package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/deployops"
)

func TestSelectedPocContainerComponentsDefaultsToFullTenantSet(t *testing.T) {
	components, errorValue := selectedPocContainerComponents(nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedComponents := []string{"admind", "capabilityd", "blueclaw", "web", "mattermostPlugins", "skills"}
	if strings.Join(components, ",") != strings.Join(expectedComponents, ",") {
		t.Fatalf("components = %v, want %v", components, expectedComponents)
	}
}

func TestSelectedPocContainerComponentsCouplesSkillsWithBlueclaw(t *testing.T) {
	components, errorValue := selectedPocContainerComponents([]string{"--components", "blueclaw"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(components, ",") != "blueclaw,skills" {
		t.Fatalf("components = %v, want blueclaw with skills coupled", components)
	}
}

func TestPocContainerCapabilityContractRequiresEveryRuntimeComponent(t *testing.T) {
	completeComponents := []string{"admind", "capabilityd", "blueclaw", "skills"}
	if !pocContainerCapabilityContractComponentsIncluded(completeComponents) {
		t.Fatal("expected complete runtime component set to allow a contract update")
	}
	for _, missingComponent := range completeComponents {
		components := []string{}
		for _, component := range completeComponents {
			if component != missingComponent {
				components = append(components, component)
			}
		}
		if pocContainerCapabilityContractComponentsIncluded(components) {
			t.Fatalf("expected missing %s to reject a contract update", missingComponent)
		}
	}
}

func TestPocContainerCapabilityContractCheckBlocksChangedPartialDeployment(t *testing.T) {
	command := pocContainerCapabilityContractCheckCommand(
		"/tmp/capability-contract.json",
		[]string{"capabilityd"},
	)
	for _, expectedFragment := range []string{
		"cmp -s",
		"capability-contract.json",
		"admind,capabilityd,blueclaw,skills",
	} {
		if !strings.Contains(command, expectedFragment) {
			t.Fatalf("contract check command is missing %q:\n%s", expectedFragment, command)
		}
	}
}

func TestSelectedPocContainerComponentsLeavesNonBlueclawUntouched(t *testing.T) {
	components, errorValue := selectedPocContainerComponents([]string{"--components", "admind"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(components, ",") != "admind" {
		t.Fatalf("components = %v, want admind", components)
	}
}

func TestSelectedPocContainerComponentsCouplesMattermostPluginsWithWeb(t *testing.T) {
	components, errorValue := selectedPocContainerComponents([]string{"--components", "web"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(components, ",") != "web,mattermostPlugins" {
		t.Fatalf("components = %v, want web with Mattermost plugins coupled", components)
	}
}

func TestNormalizePocContainerComponentMapsBlueclawPayload(t *testing.T) {
	component, errorValue := normalizePocContainerComponent("blueclawPayload")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if component != "blueclaw" {
		t.Fatalf("component = %q, want blueclaw", component)
	}
}

func TestNormalizePocContainerComponentAcceptsSkills(t *testing.T) {
	component, errorValue := normalizePocContainerComponent("skills")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if component != "skills" {
		t.Fatalf("component = %q, want skills", component)
	}
}

func TestNormalizePocContainerComponentAcceptsMattermostPlugins(t *testing.T) {
	component, errorValue := normalizePocContainerComponent("mattermostPlugins")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if component != "mattermostPlugins" {
		t.Fatalf("component = %q, want mattermostPlugins", component)
	}
}

func TestPocContainerOverlayDockerfileCopiesMattermostPlugins(t *testing.T) {
	overlayDocument := pocContainerOverlayDockerfile("internkim-poc-tenant:base-before-deploy")
	if !strings.Contains(overlayDocument, "COPY mattermost-plugins /opt/internkim/mattermost-plugins") {
		t.Fatalf("overlay missing Mattermost plugin copy:\n%s", overlayDocument)
	}
	if !strings.Contains(overlayDocument, "COPY --chmod=0755 bin/blueclaw-llmd /usr/local/bin/blueclaw-llmd") {
		t.Fatalf("overlay missing LLMD copy:\n%s", overlayDocument)
	}
}

func TestPocContainerLLMDArtifactUsesCanonicalPath(t *testing.T) {
	artifactPath := pocContainerLLMDArtifactPath("/tmp/internkim")
	expectedPath := filepath.Join("/tmp/internkim", ".dependency", "blueclaw-llmd", "blueclaw-llmd")
	if artifactPath != expectedPath {
		t.Fatalf("LLMD artifact path = %q, want %q", artifactPath, expectedPath)
	}
}

func TestPocContainerDockerfileCopiesLLMD(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "poc", "tenant", "Dockerfile"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	dockerfile := string(document)
	for _, expectedFragment := range []string{
		"COPY bin/blueclaw-llmd /usr/local/bin/blueclaw-llmd",
		"chmod 0755 /usr/local/bin/internkim-capabilityd /usr/local/bin/blueclaw /usr/local/bin/blueclaw-llmd",
	} {
		if !strings.Contains(dockerfile, expectedFragment) {
			t.Fatalf("Dockerfile missing %q:\n%s", expectedFragment, dockerfile)
		}
	}
}

func TestPocContainerEntrypointStartsLLMDBeforeCapabilityd(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("..", "..", "poc", "tenant", "entrypoint.sh"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	entrypoint := string(document)
	startLLMDIndex := strings.Index(entrypoint, "blueclaw-llmd &")
	waitLLMDIndex := strings.Index(entrypoint, "blueclaw LLMD health")
	startCapabilitydIndex := strings.Index(entrypoint, "internkim-capabilityd \\")
	if startLLMDIndex < 0 || waitLLMDIndex < 0 || startCapabilitydIndex < 0 || startLLMDIndex >= waitLLMDIndex || waitLLMDIndex >= startCapabilitydIndex {
		t.Fatalf("entrypoint must start and health-check LLMD before capabilityd:\n%s", entrypoint)
	}
	for _, expectedFragment := range []string{
		"BLUECLAW_LLMD_AUTH_KEY_PATH=\"${llmdAuthKeyPath}\"",
		"BLUECLAW_LLMD_SOCKET_PATH=\"${llmdSocketPath}\"",
		"OPENROUTER_API_KEY_PATH=/secrets/openrouter-key",
		"--llmd-socket \"${llmdSocketPath}\"",
		"--llmd-auth-key \"${llmdAuthKeyPath}\"",
		"trap shutdown INT TERM EXIT",
		"rm -f \"${llmdAuthKeyPath}\" \"${llmdAuthKeyTemporaryPath}\"",
	} {
		if !strings.Contains(entrypoint, expectedFragment) {
			t.Fatalf("entrypoint missing %q:\n%s", expectedFragment, entrypoint)
		}
	}
	for _, forbiddenFragment := range []string{"BLUECLAW_LLMD_AUTH_KEY=", "OPENROUTER_API_KEY=\""} {
		if strings.Contains(entrypoint, forbiddenFragment) {
			t.Fatalf("entrypoint must not pass secret values in environment: %q", forbiddenFragment)
		}
	}
}
func TestPocContainerRecreateCommandUsesAppleContainer(t *testing.T) {
	target := deployops.Target{ImageTag: "internkim-poc-tenant:flow"}
	command := pocContainerRecreateCommand(target)
	requiredFragments := []string{
		"container build --platform linux/arm64",
		"tenant_count=\"$(find config -type d -name 'tenant_[0-9][0-9]*'",
		"python3 start-poc.py \"$tenant_count\"",
		"python3 restart-tunnel.py",
		"TENANT_IMAGE='internkim-poc-tenant:flow'",
		"PYTHONUNBUFFERED=1",
		"base-before-deploy",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(command, fragment) {
			t.Fatalf("command missing %q:\n%s", fragment, command)
		}
	}
	for _, forbiddenFragment := range []string{"docker build", "docker compose"} {
		if strings.Contains(command, forbiddenFragment) {
			t.Fatalf("command unexpectedly contains %q:\n%s", forbiddenFragment, command)
		}
	}
}

func TestPocContainerBaseImageTagReplacesOnlyTrailingTag(t *testing.T) {
	cases := map[string]string{
		"internkim-poc-tenant:flow":              "internkim-poc-tenant:base-before-deploy",
		"registry.local:5000/internkim/poc:flow": "registry.local:5000/internkim/poc:base-before-deploy",
		"internkim-poc-tenant":                   "internkim-poc-tenant:base-before-deploy",
	}
	for imageTag, expectedBaseImageTag := range cases {
		baseImageTag := pocContainerBaseImageTag(imageTag)
		if baseImageTag != expectedBaseImageTag {
			t.Fatalf("base image tag for %q = %q, want %q", imageTag, baseImageTag, expectedBaseImageTag)
		}
	}
}

func TestPocContainerSSHUsesAskpassWithoutPasswordArguments(t *testing.T) {
	t.Setenv("INTERNKIM_POC_SSH_PASSWORD", "secret-password")
	target := deployops.Target{SSHProxyCommand: "cloudflared access ssh --hostname %h"}
	arguments := pocContainerSSHBaseArguments(target)
	joinedArguments := strings.Join(arguments, "\n")
	if !strings.Contains(joinedArguments, "ProxyCommand=cloudflared access ssh --hostname %h") {
		t.Fatalf("arguments missing proxy command: %v", arguments)
	}
	for _, expectedArgument := range []string{"ServerAliveInterval=15", "ServerAliveCountMax=12"} {
		if !strings.Contains(joinedArguments, expectedArgument) {
			t.Fatalf("arguments missing %q: %v", expectedArgument, arguments)
		}
	}
	if strings.Contains(joinedArguments, "secret-password") || strings.Contains(joinedArguments, "-p") {
		t.Fatalf("ssh arguments must not contain the password: %v", arguments)
	}
	environment := strings.Join(pocContainerSSHEnvironment(), "\n")
	for _, expectedValue := range []string{
		"SSH_ASKPASS_REQUIRE=force",
		"SSH_ASKPASS=",
		"DISPLAY=internkim-poc-ssh",
	} {
		if !strings.Contains(environment, expectedValue) {
			t.Fatalf("ssh environment is missing %q: %v", expectedValue, environment)
		}
	}
}
