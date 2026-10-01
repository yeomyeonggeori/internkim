package blueclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
)

const companyRuntimeTemplatePath = "../../../host/runtime.template.json"

func TestTheMessengerThisProductRunsOnIsDeclared(t *testing.T) {
	if !capabilityprotocol.IsMessengerPlatform(BlueclawMessengerPlatform) {
		t.Fatalf("the runtime enables %q, which the protocol does not declare a messenger", BlueclawMessengerPlatform)
	}
}

func TestTheRenderedRuntimeEnablesOnlyDeclaredPlatforms(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocument(BlueclawDefaultModelName)
	if errorValue != nil {
		t.Fatalf("expected a runtime document: %v", errorValue)
	}

	for _, platform := range enabledPlatformsOf(t, []byte(document)) {
		if !capabilityprotocol.IsMessengerPlatform(platform) {
			t.Fatalf("the rendered runtime enables %q, which the protocol does not declare a messenger", platform)
		}
	}
}

func TestTheCompanyRuntimeTemplateNamesNoPlatformOfItsOwn(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Clean(companyRuntimeTemplatePath))
	if errorValue != nil {
		t.Fatalf("expected the company runtime template: %v", errorValue)
	}

	enabledPlatforms := enabledPlatformsOf(t, document)
	if len(enabledPlatforms) != 1 || enabledPlatforms[0] != "${MESSENGER_PLATFORM}" {
		t.Fatalf("expected the template to leave the platform to its deployment, got %v", enabledPlatforms)
	}
}

func enabledPlatformsOf(t *testing.T, document []byte) []string {
	t.Helper()
	runtimeConfiguration := struct {
		Connectors struct {
			Chatd struct {
				EnabledPlatforms []string `json:"enabledPlatforms"`
			} `json:"chatd"`
		} `json:"connectors"`
	}{}
	if errorValue := json.Unmarshal(document, &runtimeConfiguration); errorValue != nil {
		t.Fatalf("expected a runtime configuration document: %v", errorValue)
	}
	return runtimeConfiguration.Connectors.Chatd.EnabledPlatforms
}
