package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type addressProbe func(address string) (int, error)

func runVerifyInstallAddresses() error {
	return reportInstallAddresses(capabilities.InstallScriptURLs(), fetchAddressStatus, os.Stdout)
}

func reportInstallAddresses(addresses []string, probe addressProbe, output io.Writer) error {
	var unserved []string
	for _, address := range addresses {
		status, errorValue := probe(address)
		if errorValue != nil {
			fmt.Fprintf(output, "fail %s: %s\n", address, errorValue)
			unserved = append(unserved, address)
			continue
		}
		if status != http.StatusOK {
			fmt.Fprintf(output, "fail %s: %d\n", address, status)
			unserved = append(unserved, address)
			continue
		}
		fmt.Fprintf(output, "ok   %s\n", address)
	}
	if len(unserved) > 0 {
		return fmt.Errorf("%v %s not served; deploy web before the release that prints them", unserved, pluralIsAre(len(unserved)))
	}
	return nil
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
