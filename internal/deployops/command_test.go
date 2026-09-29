package deployops

import (
	"strings"
	"testing"
)

func TestAnOpsJobRunsTheCLIWithTheVaultItWasHanded(t *testing.T) {
	t.Setenv(FleetSecretVariable, "handed-by-the-vault")

	plan := deployCommand("/repo", "/tmp/internkim", "admind")

	if arguments := strings.Join(plan.Arguments, " "); arguments != "deploy --components admind" {
		t.Fatalf("unexpected arguments: %s", arguments)
	}
	if plan.DirectoryPath != "/repo" {
		t.Fatalf("expected repository root working directory, got %q", plan.DirectoryPath)
	}
	if !containsValue(plan.Environment, FleetSecretVariable+"=handed-by-the-vault") {
		t.Fatal("the job does not carry the environment the console was started with")
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
