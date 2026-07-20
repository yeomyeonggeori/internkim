package cli

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestFirstbootStagesLLMDCredentialsBeforeServiceActivation(t *testing.T) {
	t.Setenv(blueclaw.LocalOnlyEnvironment, "")
	document := renderFirstbootServicesSection()
	credentialInstallIndex := strings.Index(document, "install -o root -g root -m 600 "+blueclaw.LLMDAuthKeyPath+" "+blueclaw.LLMDServiceAuthKeyPath)
	serviceStartIndex := strings.Index(document, "systemctl start "+blueclaw.LLMDServiceName)
	if credentialInstallIndex < 0 || serviceStartIndex < 0 || credentialInstallIndex > serviceStartIndex {
		t.Fatalf("expected LLMD credential staging before service activation, got %s", document)
	}
	if !strings.Contains(document, "install -d -o root -g root -m 700 "+blueclaw.LLMDServiceCredentialDirectoryPath) {
		t.Fatalf("expected firstboot to create LLMD credential directory, got %s", document)
	}
}

func TestFirstbootLocalOnlyLLMDWithholdsRemoteCredentialAndNetwork(t *testing.T) {
	t.Setenv(blueclaw.LocalOnlyEnvironment, "true")
	document := renderFirstbootServicesSection()
	for _, expectedValue := range []string{
		"Environment=BLUECLAW_LLMD_LOCAL_ONLY=1",
		"rm -f " + blueclaw.LLMDServiceOpenRouterKeyPath,
		"IPAddressDeny=any",
		"IPAddressAllow=localhost",
	} {
		if !strings.Contains(document, expectedValue) {
			t.Fatalf("expected local-only firstboot to contain %q, got %s", expectedValue, document)
		}
	}
	if strings.Contains(document, "LoadCredential=-openrouter-api-key:") {
		t.Fatalf("expected local-only firstboot to withhold LLMD OpenRouter credential, got %s", document)
	}
}
