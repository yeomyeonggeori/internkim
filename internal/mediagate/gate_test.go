package mediagate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAStoredAttachmentIsAnsweredOnlyOnceKept(t *testing.T) {
	seenHost := ""
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		seenHost = request.Host
		writer.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	for _, scenario := range []struct {
		method     string
		keep       error
		wantStatus int
		wantKept   bool
	}{
		{http.MethodPut, nil, http.StatusOK, true},
		{http.MethodPut, errors.New("the bucket is unreachable"), http.StatusServiceUnavailable, true},
		{http.MethodGet, errors.New("never asked"), http.StatusOK, false},
	} {
		isAsked := false
		gate := Gate{Upstream: address, KeepFiles: func(ctx context.Context) error {
			isAsked = true
			return scenario.keep
		}}
		server := httptest.NewServer(gate.Handler())
		request, _ := http.NewRequest(scenario.method, server.URL+"/media/object", strings.NewReader("picture"))
		request.Host = "127.0.0.1:9000"
		response, errorValue := http.DefaultClient.Do(request)
		server.Close()
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if response.StatusCode != scenario.wantStatus || isAsked != scenario.wantKept {
			t.Errorf("%s with keep %v answered %d, asked to keep %v", scenario.method, scenario.keep, response.StatusCode, isAsked)
		}
		if seenHost != "127.0.0.1:9000" {
			t.Errorf("the store saw Host %q; a signed request must reach it with the host it was signed for", seenHost)
		}
	}
}
