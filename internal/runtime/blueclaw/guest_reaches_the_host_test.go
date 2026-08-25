package blueclaw

import (
	"encoding/json"
	"strings"
	"testing"
)

// chatd runs on this machine and the agent runs in the guest, where a loopback
// address is the guest's own. Naming chatd at loopback is refused with nothing
// in the configuration to say why, which is how buzz went silent: the guest was
// never told where chatd was, so it fell back to a loopback of its own.
func TestTheGuestIsToldWhereChatdIsRatherThanFallingBackToItsOwnLoopback(t *testing.T) {
	for _, gateway := range []string{"", "172.31.0.1", "10.9.8.1"} {
		document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{OutboundGuestGateway: gateway})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		endpoint := chatdEndpointIn(t, document)
		expectedGateway := gateway
		if expectedGateway == "" {
			expectedGateway = "172.31.0.1"
		}
		if endpoint != "http://"+expectedGateway+":"+ChatdListenPort {
			t.Fatalf("with gateway %q the guest is told to call chatd at %q", gateway, endpoint)
		}
	}
}

// The address the guest is given and the address chatd binds are the same one,
// or the call reaches a port nothing answers on.
func TestChatdAnswersWhereTheGuestIsToldToLookForIt(t *testing.T) {
	if !strings.Contains(ChatdEndpoint, ChatdListenHostname) {
		t.Fatalf("the host calls %q while chatd binds %q", ChatdEndpoint, ChatdListenHostname)
	}
	unit := ChatdServiceUnit("")
	if !strings.Contains(unit, "CHATD_LISTEN_HOSTNAME="+ChatdListenHostname) {
		t.Fatalf("the chatd unit does not bind %q:\n%s", ChatdListenHostname, unit)
	}
}

// Buzz reaches the agent only through chatd, so a guest that is not told chatd
// carries the platform has no way to answer anyone on it.
func TestBuzzIsNamedAsAPlatformChatdCarries(t *testing.T) {
	document, errorValue := BlueclawRuntimeConfigDocumentWithOptions(RuntimeConfigOptions{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var configuration struct {
		Connectors struct {
			Chatd struct {
				EnabledPlatforms []string `json:"enabledPlatforms"`
			} `json:"chatd"`
		} `json:"connectors"`
	}
	if errorValue := json.Unmarshal([]byte(document), &configuration); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, platform := range configuration.Connectors.Chatd.EnabledPlatforms {
		if platform == "buzz" {
			return
		}
	}
	t.Fatalf("chatd carries %v, and buzz is not among them", configuration.Connectors.Chatd.EnabledPlatforms)
}

func chatdEndpointIn(t *testing.T, document string) string {
	t.Helper()
	var configuration struct {
		Connectors struct {
			Chatd struct {
				Endpoint string `json:"endpoint"`
			} `json:"chatd"`
		} `json:"connectors"`
	}
	if errorValue := json.Unmarshal([]byte(document), &configuration); errorValue != nil {
		t.Fatal(errorValue)
	}
	return configuration.Connectors.Chatd.Endpoint
}
