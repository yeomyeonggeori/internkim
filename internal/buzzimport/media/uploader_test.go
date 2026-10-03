package media

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUploaderAsksTheRelayByItsPublicHostWhileDiallingLocally(t *testing.T) {
	var askedHost string
	store := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		askedHost = request.Host
		json.NewEncoder(writer).Encode(map[string]any{"url": "https://company.example.com/abc", "sha256": "abc", "size": 3, "type": "image/png"})
	}))
	defer store.Close()

	uploader := Uploader{HTTPBaseURL: store.URL, RelayHost: "company.example.com"}
	if _, errorValue := uploader.Upload(context.Background(), "1111111111111111111111111111111111111111111111111111111111111111", []byte("png"), "image/png"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if askedHost != "company.example.com" {
		t.Fatalf("the upload asked for host %q, want the relay's public host", askedHost)
	}
}
