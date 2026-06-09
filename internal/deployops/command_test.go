package deployops

import (
	"strings"
	"testing"
)

func TestDeployCommandUsesProfileAndNode(t *testing.T) {
	target := Target{
		Profile:      "pilot",
		NodeArgument: "1",
	}

	plan := deployCommand("/repo", "/tmp/internkim", target, "admind")
	arguments := strings.Join(plan.Arguments, " ")
	if arguments != "deploy --components admind --node 1" {
		t.Fatalf("unexpected arguments: %s", arguments)
	}
	if plan.DirectoryPath != "/repo" {
		t.Fatalf("expected repository root working directory, got %q", plan.DirectoryPath)
	}
	if !containsValue(plan.Environment, "INTERNKIM_PROFILE=pilot") {
		t.Fatalf("expected profile environment in %#v", plan.Environment)
	}
}

func TestRedactRemovesSecrets(t *testing.T) {
	value := Redact("fleet_secret=abcdefghijklmnopqrstuvwxyz012345 url=https://pilot-01.example.test")
	if strings.Contains(value, "abcdefghijklmnopqrstuvwxyz012345") {
		t.Fatalf("secret was not redacted: %s", value)
	}
	if !strings.Contains(value, "https://pilot-01.example.test") {
		t.Fatalf("public URL should remain visible: %s", value)
	}
}

func containsValue(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}

func TestCommandEnvironmentKeepsExistingEnvironment(t *testing.T) {
	t.Setenv("DEPLOYOPS_TEST_ENV", "present")
	environment := commandEnvironment(Target{})
	if !containsValue(environment, "DEPLOYOPS_TEST_ENV=present") {
		t.Fatalf("expected process environment to be preserved, got %d entries", len(environment))
	}
	if containsValue(environment, "INTERNKIM_PROFILE=") {
		t.Fatal("empty profile should not be exported")
	}
}
