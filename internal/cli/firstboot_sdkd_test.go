package cli

import (
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func TestFirstbootStagesSDKDCredentialsBeforeServiceActivation(t *testing.T) {
	t.Setenv(blueclaw.LocalOnlyEnvironment, "")
	document := renderFirstbootServicesSection()
	credentialInstallIndex := strings.Index(document, "install -o root -g root -m 600 "+blueclaw.SDKDAuthKeyPath+" "+blueclaw.SDKDServiceAuthKeyPath)
	serviceStartIndex := strings.Index(document, "systemctl start "+blueclaw.SDKDServiceName)
	if credentialInstallIndex < 0 || serviceStartIndex < 0 || credentialInstallIndex > serviceStartIndex {
		t.Fatalf("expected SDKD credential staging before service activation, got %s", document)
	}
	if !strings.Contains(document, "install -d -o root -g root -m 700 "+blueclaw.SDKDServiceCredentialDirectoryPath) {
		t.Fatalf("expected firstboot to create SDKD credential directory, got %s", document)
	}
}

func TestFirstbootLocalOnlySDKDWithholdsRemoteCredentialAndNetwork(t *testing.T) {
	t.Setenv(blueclaw.LocalOnlyEnvironment, "true")
	document := renderFirstbootServicesSection()
	for _, expectedValue := range []string{
		"Environment=BLUECLAW_SDKD_LOCAL_ONLY=1",
		"rm -f " + blueclaw.SDKDServiceOpenRouterKeyPath,
		"IPAddressDeny=any",
		"IPAddressAllow=localhost",
	} {
		if !strings.Contains(document, expectedValue) {
			t.Fatalf("expected local-only firstboot to contain %q, got %s", expectedValue, document)
		}
	}
	if strings.Contains(document, "LoadCredential=-openrouter-api-key:") {
		t.Fatalf("expected local-only firstboot to withhold SDKD OpenRouter credential, got %s", document)
	}
}
