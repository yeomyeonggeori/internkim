package admind

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBlueclawPolicyUserRecordsServesCacheOnRequestFailure(t *testing.T) {
	policyServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		io.WriteString(responseWriter, `{"people":[{"displayName":"Probe","emails":["probe@internkim.test"]}]}`)
	}))
	service := &Service{Configuration: Configuration{BlueclawBaseURL: policyServer.URL}, HTTPClient: policyServer.Client()}
	liveRecords := service.blueclawPolicyUserRecords(context.Background())
	if len(liveRecords) != 1 || liveRecords[0].Email != "probe@internkim.test" {
		t.Fatalf("unexpected live records: %+v", liveRecords)
	}
	policyServer.Close()
	cachedRecords := service.blueclawPolicyUserRecords(context.Background())
	if len(cachedRecords) != 1 || cachedRecords[0].Email != "probe@internkim.test" {
		t.Fatalf("expected cached records after request failure, got %+v", cachedRecords)
	}
}
