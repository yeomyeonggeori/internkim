package cli

import (
	"strings"
	"testing"
)

func TestAdminLoopbackCommandReachesTheDeviceWithoutAHostname(t *testing.T) {
	command := adminLoopbackCommand("GET", "/admin/api/updates/status", nil)

	if !strings.Contains(command, "http://127.0.0.1:18080/admin/api/updates/status") {
		t.Fatalf("expected the loopback address, got %s", command)
	}
	for _, forbidden := range []string{"intern.kim", "https://", "cloudflared"} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("a loopback request names no public host, found %q in %s", forbidden, command)
		}
	}
}

func TestAdminLoopbackCommandCarriesABodyThroughBase64(t *testing.T) {
	command := adminLoopbackCommand("POST", "/admin/api/updates/apply", []byte(`{"releaseID":"a b'c"}`))

	if strings.Contains(command, "a b'c") {
		t.Fatalf("the body must not reach the shell verbatim, got %s", command)
	}
	if !strings.Contains(command, "base64 -d") {
		t.Fatalf("expected the body to be decoded on the device, got %s", command)
	}
	if !strings.Contains(command, "--data-binary @-") {
		t.Fatalf("expected the decoded body to be piped into the request, got %s", command)
	}
}

func TestAdminLoopbackCommandSendsNoBodyWhenThereIsNone(t *testing.T) {
	command := adminLoopbackCommand("GET", "/admin/api/updates/status", nil)

	for _, forbidden := range []string{"base64", "--data-binary", "Content-Type"} {
		if strings.Contains(command, forbidden) {
			t.Fatalf("a request with no body carries none, found %q in %s", forbidden, command)
		}
	}
}

func TestAdminLoopbackCommandFailsLoudlyOnAnErrorStatus(t *testing.T) {
	command := adminLoopbackCommand("POST", "/admin/api/updates/apply", []byte(`{}`))

	if !strings.Contains(command, "--fail-with-body") {
		t.Fatalf("a 4xx must be an error the caller sees, with what admind said, got %s", command)
	}
}
