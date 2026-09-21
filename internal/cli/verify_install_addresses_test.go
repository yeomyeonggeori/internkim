package cli

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestEveryAddressAdmindPrintsIsVerified(t *testing.T) {
	probed := map[string]bool{}
	var output bytes.Buffer
	probe := func(address string) (int, error) {
		probed[address] = true
		return http.StatusOK, nil
	}

	if errorValue := reportInstallAddresses(capabilities.InstallScriptURLs(), probe, &output); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, address := range []string{capabilities.InstallScriptURL(), capabilities.RetiredCompanionInstallScriptURL()} {
		if !probed[address] {
			t.Fatalf("%s was never probed", address)
		}
	}
	if !strings.Contains(capabilities.CompanionInstallCommand(), capabilities.InstallScriptURL()) {
		t.Fatal("the command admind prints names an address this check does not verify")
	}
}

func TestAnAddressThatIsNotServedRefusesTheDeploy(t *testing.T) {
	for name, probe := range map[string]addressProbe{
		"a missing file":      func(string) (int, error) { return http.StatusNotFound, nil },
		"an unreachable host": func(string) (int, error) { return 0, fmt.Errorf("no such host") },
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			errorValue := reportInstallAddresses([]string{"https://example.test/install.sh"}, probe, &output)
			if errorValue == nil || !strings.Contains(errorValue.Error(), "deploy web before") {
				t.Fatalf("%s returned %v", name, errorValue)
			}
			if !strings.Contains(output.String(), "fail https://example.test/install.sh") {
				t.Fatalf("the report does not name the address: %q", output.String())
			}
		})
	}
}
