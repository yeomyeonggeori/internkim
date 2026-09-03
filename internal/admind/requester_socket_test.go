package admind

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestARequesterHeaderOnTheTCPListenerNamesNobody(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	tcpServer := httptest.NewServer(service.router())
	defer tcpServer.Close()

	response := postQuickTaskAsRequester(t, tcpServer.Client(), tcpServer.URL, "member@example.com")
	defer response.Body.Close()

	if response.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("the port named a person it has no right to: %d %s", response.StatusCode, string(body))
	}
}

func TestARequesterHeaderOnTheSocketNamesThePerson(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	socketPath, socketClient := serveOnARequesterSocket(t, service)

	response := postQuickTaskAsRequester(t, socketClient, "http://internkim", "member@example.com")
	defer response.Body.Close()

	if response.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("%s refused the person it asserted: %s", socketPath, string(body))
	}
}

func TestTheRequesterSocketIsReachableOnlyByItsOwnGroup(t *testing.T) {
	socketPath, _ := serveOnARequesterSocket(t, newTaskAuthorizationTestService(t))

	information, errorValue := os.Stat(socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if mode := information.Mode().Perm(); mode != requesterSocketMode {
		t.Fatalf("%s is mode %#o, which is not %#o", socketPath, mode, requesterSocketMode)
	}
}

func serveOnARequesterSocket(t *testing.T, service *Service) (string, *http.Client) {
	t.Helper()
	directoryPath, errorValue := os.MkdirTemp("/tmp", "ik-admind-*")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directoryPath) })
	socketPath := filepath.Join(directoryPath, "admind.sock")
	listener, errorValue := listenOnRequesterSocket(socketPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	server := &http.Server{Handler: markRequestsAsAssertedByTheListener(service.router())}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	socketClient := &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _ string, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}}
	return socketPath, socketClient
}

func postQuickTaskAsRequester(t *testing.T, client *http.Client, baseURL string, requesterEmail string) *http.Response {
	t.Helper()
	request, errorValue := http.NewRequest(http.MethodPost, baseURL+quickTaskPath, strings.NewReader(`{"prompt":"보고서"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Header.Set(requesterEmailHeader, requesterEmail)
	response, errorValue := client.Do(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}
