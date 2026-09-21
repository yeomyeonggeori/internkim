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

func TestAnAddressThatDoesNotAnswer200RefusesTheDeployForTheReasonItFound(t *testing.T) {
	missing := "https://example.test/install.sh"
	unreachable := "https://example.test/companion/install.sh"
	probe := func(address string) (int, error) {
		if address == missing {
			return http.StatusNotFound, nil
		}
		return 0, fmt.Errorf("no such host")
	}

	for _, verdict := range []struct {
		name           string
		addresses      []string
		says           string
		neverSays      string
		expectedProbes map[string]int
	}{
		{
			name:           "an address the web app does not serve",
			addresses:      []string{missing},
			says:           "deploy web before",
			neverSays:      "not evidence that web is behind",
			expectedProbes: map[string]int{missing: 1},
		},
		{
			name:           "an address nothing could reach",
			addresses:      []string{unreachable},
			says:           "not evidence that web is behind",
			neverSays:      "deploy web before",
			expectedProbes: map[string]int{unreachable: installAddressAttempts},
		},
		{
			name:           "one of each",
			addresses:      []string{missing, unreachable},
			says:           "deploy web before",
			neverSays:      "",
			expectedProbes: map[string]int{missing: 1, unreachable: installAddressAttempts},
		},
	} {
		t.Run(verdict.name, func(t *testing.T) {
			probes := map[string]int{}
			counting := func(address string) (int, error) {
				probes[address]++
				return probe(address)
			}
			var output bytes.Buffer

			errorValue := reportInstallAddresses(verdict.addresses, counting, &output)

			if errorValue == nil {
				t.Fatal("the deploy was allowed to proceed")
			}
			if !strings.Contains(errorValue.Error(), verdict.says) {
				t.Fatalf("the refusal does not say %q: %v", verdict.says, errorValue)
			}
			if verdict.neverSays != "" && strings.Contains(errorValue.Error(), verdict.neverSays) {
				t.Fatalf("the refusal claims %q, which it did not establish: %v", verdict.neverSays, errorValue)
			}
			if verdict.name == "one of each" && !strings.Contains(errorValue.Error(), "not evidence that web is behind") {
				t.Fatalf("the refusal drops one of the two reasons: %v", errorValue)
			}
			for address, expected := range verdict.expectedProbes {
				if probes[address] != expected {
					t.Fatalf("%s was probed %d times, not %d", address, probes[address], expected)
				}
				if !strings.Contains(output.String(), "fail "+address) {
					t.Fatalf("the report does not name %s: %q", address, output.String())
				}
			}
		})
	}
}
