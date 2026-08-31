package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fleetAnswering(t *testing.T, status int, body string) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/task/api/state" {
			http.NotFound(responseWriter, request)
			return
		}
		responseWriter.WriteHeader(status)
		_, _ = responseWriter.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestAFleetReadingTheCompanyRecordIsAccepted(t *testing.T) {
	address := fleetAnswering(t, http.StatusOK, `{"source":"central","tasks":[]}`)

	if errorValue := refuseAFleetWithoutACompany(context.Background(), address+"/"); errorValue != nil {
		t.Fatalf("a fleet that belongs to a company runs scenarios, got %v", errorValue)
	}
}

func TestAFleetReadingItsOwnStoreIsRefused(t *testing.T) {
	address := fleetAnswering(t, http.StatusOK, `{"source":"sqlite","tasks":[]}`)

	errorValue := refuseAFleetWithoutACompany(context.Background(), address)
	if errorValue == nil {
		t.Fatal("a scenario against the device's own store proves nothing and is refused")
	}
	if !strings.Contains(errorValue.Error(), "sqlite") {
		t.Fatalf("the refusal names what the board read, got %q", errorValue.Error())
	}
}

func TestAFleetThatWillNotSayIsRefused(t *testing.T) {
	for _, answer := range []struct {
		name   string
		status int
		body   string
	}{
		{"a board that errors", http.StatusInternalServerError, ""},
		{"a board naming no source", http.StatusOK, `{"tasks":[]}`},
		{"a board answering something else", http.StatusOK, `not json`},
	} {
		t.Run(answer.name, func(t *testing.T) {
			address := fleetAnswering(t, answer.status, answer.body)
			if refuseAFleetWithoutACompany(context.Background(), address) == nil {
				t.Fatal("a fleet that will not say where its board comes from is refused")
			}
		})
	}
}
