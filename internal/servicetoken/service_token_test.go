package servicetoken

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestARequestCarriesTheTokenItsServiceIsPointedAt(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "service-token")
	if errorValue := os.WriteFile(tokenPath, []byte("token-of-the-host-services\n"), 0o440); errorValue != nil {
		t.Fatal(errorValue)
	}
	request, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18090/v1/platform/buzz/message.post", nil)

	if errorValue := Authorize(request, tokenPath); errorValue != nil {
		t.Fatal(errorValue)
	}

	if request.Header.Get("Authorization") != "Bearer token-of-the-host-services" {
		t.Fatalf("the request carried %q", request.Header.Get("Authorization"))
	}
}

func TestATokenThatCannotBeReadStopsTheRequest(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "http://127.0.0.1:18090/v1/platform/buzz/message.post", nil)

	if Authorize(request, filepath.Join(t.TempDir(), "missing")) == nil {
		t.Fatal("a token nobody could read was taken as sent")
	}
}
