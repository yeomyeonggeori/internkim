package capabilityd

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func constantDeclaredIn(t *testing.T, path string, name string) string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.FromSlash(path))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	match := regexp.MustCompile(`export const ` + name + ` = ([^;]+);`).FindSubmatch(source)
	if match == nil {
		t.Fatalf("%s declares no %s, so this guard is reading the wrong source", path, name)
	}
	return strings.Trim(string(match[1]), "'")
}

func numberDeclaredIn(t *testing.T, path string, name string) int {
	t.Helper()
	number, errorValue := strconv.Atoi(strings.ReplaceAll(constantDeclaredIn(t, path, name), "_", ""))
	if errorValue != nil {
		t.Fatalf("%s in %s is not a plain number: %v", name, path, errorValue)
	}
	return number
}

func TestCapabilitydReadsDesktopWorkAnswersAsTheRelayAndTheGatewayGiveThem(t *testing.T) {
	const relay = "../../host/relay/desktop-work.ts"
	const gateway = "../../workers/connection-gateway/src/company-connection.ts"

	if path := constantDeclaredIn(t, relay, "desktopWorkPath"); path != relayDesktopWorkPath {
		t.Errorf("the relay takes desktop work at %s and capabilityd posts it to %s", path, relayDesktopWorkPath)
	}
	if status := numberDeclaredIn(t, relay, "noGatewayStatus"); status != relayHasNoGatewayStatus {
		t.Errorf("a relay with no gateway answers %d and capabilityd reads %d as that", status, relayHasNoGatewayStatus)
	}
	if status := numberDeclaredIn(t, relay, "gatewayUnreachedStatus"); status != relayDidNotReachGatewayStatus {
		t.Errorf("a relay that got no answer from the gateway answers %d and capabilityd reads %d as that", status, relayDidNotReachGatewayStatus)
	}
	if status := numberDeclaredIn(t, gateway, "noBoxTookTheWorkStatus"); status != noDesktopTookTheWorkStatus {
		t.Errorf("the gateway answers %d when no desktop took the work and capabilityd reads %d as that", status, noDesktopTookTheWorkStatus)
	}
	if bound := time.Duration(numberDeclaredIn(t, gateway, "workerCallBoundMilliseconds")) * time.Millisecond; desktopWorkPatience <= bound {
		t.Errorf("capabilityd waits %s for desktop work and gives up before the gateway answers at %s", desktopWorkPatience, bound)
	}
}
