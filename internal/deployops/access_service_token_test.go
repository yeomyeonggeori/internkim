package deployops

import (
	"net/http"
	"testing"
)

func TestCloudflareAccessServiceTokenComesFromTheEnvironment(t *testing.T) {
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_ID", " id.access ")
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_SECRET", " secret ")
	clientID, clientSecret := cloudflareAccessServiceToken()
	if clientID != "id.access" || clientSecret != "secret" {
		t.Fatalf("expected the vault's credentials trimmed, got %q %q", clientID, clientSecret)
	}
}

func TestCloudflareAccessServiceTokenAbsentFromTheEnvironmentYieldsNothing(t *testing.T) {
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_ID", "")
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_SECRET", "")
	clientID, clientSecret := cloudflareAccessServiceToken()
	if clientID != "" || clientSecret != "" {
		t.Fatal("an empty environment must yield no credential, so the login-token path serves instead")
	}
}

func TestAttachCloudflareAccessSendsTheServiceTokenHeaders(t *testing.T) {
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_ID", "id.access")
	t.Setenv("INTERNKIM_CF_ACCESS_CLIENT_SECRET", "secret")
	request, errorValue := http.NewRequest(http.MethodGet, "https://example.test/admin/api/health", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	AttachCloudflareAccess(request)
	if request.Header.Get("CF-Access-Client-Id") != "id.access" ||
		request.Header.Get("CF-Access-Client-Secret") != "secret" {
		t.Fatal("the request went out without the service token the environment held")
	}
}
