package cli

import (
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

func TestSelectedPocContainerComponentsLeavesNonBlueclawUntouched(t *testing.T) {
	components, errorValue := selectedPocContainerComponents([]string{"--components", "web,admind"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Join(components, ",") != "web,admind" {
		t.Fatalf("components = %v, want web,admind without skills", components)
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

func TestPocContainerSSHArgumentsIncludeProxyCommand(t *testing.T) {
	t.Setenv("INTERNKIM_POC_SSH_PASSWORD", "secret-password")
	target := deployops.Target{SSHProxyCommand: "cloudflared access ssh --hostname %h"}
	arguments := pocContainerSSHBaseArguments(target, "ssh")
	joinedArguments := strings.Join(arguments, "\n")
	if !strings.Contains(joinedArguments, "ProxyCommand=cloudflared access ssh --hostname %h") {
		t.Fatalf("arguments missing proxy command: %v", arguments)
	}
	if strings.Contains(strings.Join(redactPocCommandArguments(arguments), " "), "secret-password") {
		t.Fatalf("redacted arguments leaked password: %v", redactPocCommandArguments(arguments))
	}
}
