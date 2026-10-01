package cli

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

// The device answers through Cloudflare over a WAN whose round trip spikes
// past ten seconds now and then, which tools/deploy-main's device_revision_of
// already allows for; one stalled read is not an address that is missing.
const installAddressAttempts = 3

type addressProbe func(address string) (int, error)

func runVerifyInstallAddresses() error {
	return reportInstallAddresses(capabilities.InstallScriptURLs(), fetchAddressStatus, os.Stdout)
}

func reportInstallAddresses(addresses []string, probe addressProbe, output io.Writer) error {
	var unserved, unreachable []string
	for _, address := range addresses {
		status, errorValue := probeUntilAnswered(address, probe, output)
		switch {
		case errorValue != nil:
			fmt.Fprintf(output, "fail %s: %s\n", address, errorValue)
			unreachable = append(unreachable, address)
		case status != http.StatusOK:
			fmt.Fprintf(output, "fail %s: %d\n", address, status)
			unserved = append(unserved, address)
		default:
			fmt.Fprintf(output, "ok   %s\n", address)
		}
	}
	return installAddressVerdict(unserved, unreachable)
}

func probeUntilAnswered(address string, probe addressProbe, output io.Writer) (int, error) {
	var status int
	var errorValue error
	for attempt := 1; attempt <= installAddressAttempts; attempt++ {
		if status, errorValue = probe(address); errorValue == nil {
			return status, nil
		}
		if attempt < installAddressAttempts {
			fmt.Fprintf(output, "wait %s did not answer (%s); asking again\n", address, errorValue)
		}
	}
	return 0, errorValue
}

func installAddressVerdict(unserved, unreachable []string) error {
	var refusals []string
	if len(unserved) > 0 {
		refusals = append(refusals, fmt.Sprintf(
			"%v %s not served; deploy web before the release that prints them",
			unserved, pluralIsAre(len(unserved))))
	}
	if len(unreachable) > 0 {
		refusals = append(refusals, fmt.Sprintf(
			"%v did not answer in %d attempts, which is not evidence that web is behind; retry, or look at the network",
			unreachable, installAddressAttempts))
	}
	if len(refusals) == 0 {
		return nil
	}
	return errors.New(strings.Join(refusals, ". "))
}

func pluralIsAre(count int) string {
	if count == 1 {
		return "is"
	}
	return "are"
}

func fetchAddressStatus(address string) (int, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	request, errorValue := http.NewRequest(http.MethodGet, address, nil)
	if errorValue != nil {
		return 0, errorValue
	}
	request.Header.Set("User-Agent", "internkim-verify-install-addresses")
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return 0, errorValue
	}
	defer response.Body.Close()
	return response.StatusCode, nil
}
