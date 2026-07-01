package admind

import (
	"net/http"
	"testing"
	"time"
)

func TestAdmindHTTPClientBoundsResponseHeaderWait(t *testing.T) {
	transport, ok := admindHTTPClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("admind http client transport is %T, want *http.Transport", admindHTTPClient.Transport)
	}
	if transport.ResponseHeaderTimeout != 30*time.Second {
		t.Fatalf("ResponseHeaderTimeout is %v, want 30s to bound stuck upstream connections", transport.ResponseHeaderTimeout)
	}
}
