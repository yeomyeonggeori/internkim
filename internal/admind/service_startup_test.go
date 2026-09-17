package admind

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunServesRequesterMemoryWhileSiteCleanupWaits(t *testing.T) {
	service, _ := newTestSiteService(t)
	publishStaticTestSite(t, service, "startup-static")
	socketDirectory, errorValue := os.MkdirTemp("/tmp", "admind-startup-")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDirectory) })
	requesterSocketPath := filepath.Join(socketDirectory, "admind.sock")
	service.Configuration.ListenAddress = "127.0.0.1:0"
	service.Configuration.ListenSocketPath = requesterSocketPath
	service.Configuration.DatabasePath = ":memory:"
	service.Configuration.CentralPlaneAppURL = "https://app.example.test"
	service.Configuration.CentralPlaneAppURLPath = writeTestFile(t, "https://app.example.test")
	service.Configuration.CentralPlaneProjectURL = companyProjectURLForTest
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.BlueclawBaseURL = "http://blueclaw.local"
	service.Configuration.FleetIDPath = writeTestFile(t, "device-1")
	service.Configuration.FleetSecretPath = writeTestFile(t, "secret-1")
	service.Configuration.CentralPlaneAgentKeyPath = writeTestFile(t, "test-agent-key")
	holdWorkspaceSettingsForTest(service, "Asia/Seoul", workspaceLanguageKorean)
	cleanupStarted := make(chan struct{})
	releaseCleanup := make(chan struct{})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if name == "systemctl" && strings.Contains(strings.Join(arguments, " "), "disable --now") {
			close(cleanupStarted)
			select {
			case <-releaseCleanup:
				return []byte("ok"), nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return []byte("ok"), nil
	}
	personaSeeded := make(chan struct{})
	service.HTTPClient = memoryFactsClient(personaSeeded)
	contextValue, cancel := context.WithCancel(context.Background())
	runError := make(chan error, 1)
	go func() { runError <- service.Run(contextValue) }()
	t.Cleanup(func() {
		cancel()
		select {
		case errorValue := <-runError:
			if errorValue != nil {
				t.Error(errorValue)
			}
		case <-time.After(5 * time.Second):
			t.Error("admind did not stop")
		}
	})

	awaitFile(t, requesterSocketPath)
	select {
	case <-cleanupStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("site cleanup did not block startup")
	}
	select {
	case <-personaSeeded:
	case <-time.After(5 * time.Second):
		t.Fatal("persona fixture was not seeded")
	}

	response := requesterMemoryFacts(t, requesterSocketPath)
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("requester memory status = %d", response.StatusCode)
	}
	var facts map[string]any
	if errorValue := json.NewDecoder(response.Body).Decode(&facts); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, exists := facts["facts"]; !exists {
		t.Fatalf("requester memory response = %+v", facts)
	}
	close(releaseCleanup)
}

func TestRunReturnsConfiguredRequesterSocketBindError(t *testing.T) {
	service, _ := newTestSiteService(t)
	service.Configuration.ListenAddress = "127.0.0.1:0"
	parentPath := filepath.Join(t.TempDir(), "ordinary-file")
	service.Configuration.ListenSocketPath = filepath.Join(parentPath, "admind.sock")
	if errorValue := os.WriteFile(parentPath, []byte("occupied"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.RunCommand = func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	service.HTTPClient = memoryFactsClient(nil)
	contextValue, cancel := context.WithCancel(context.Background())
	defer cancel()
	errorValue := service.Run(contextValue)
	if errorValue == nil {
		t.Fatal("expected requester socket bind error")
	}
}

func TestSiteRuntimeReconcileWaitHonorsContextCancellation(t *testing.T) {
	service, _ := newTestSiteService(t)
	staticSite := publishStaticTestSite(t, service, "startup-cancel")
	started := make(chan struct{})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if name == "systemctl" && strings.Contains(strings.Join(arguments, " "), "disable --now") {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return []byte("ok"), nil
	}
	startupContext, cancelStartup := context.WithCancel(context.Background())
	defer cancelStartup()
	service.startSiteRuntimeReconcile(startupContext)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("site cleanup did not start")
	}
	requestContext, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	if errorValue := service.reconcileSitePocketBaseRuntime(requestContext, staticSite, staticSite.CurrentVersionID); !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("wait error = %v, want context canceled", errorValue)
	}
	if errorValue := service.ensureSitePocketBaseRunning(requestContext, staticSite); !errors.Is(errorValue, context.Canceled) {
		t.Fatalf("start error = %v, want context canceled", errorValue)
	}
	cancelStartup()
	<-service.siteRuntimeStartupDone
}

func awaitFile(t *testing.T, path string) {
	t.Helper()
	for attempt := 0; attempt < 100; attempt++ {
		if _, errorValue := os.Stat(path); errorValue == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("file did not appear: %s", path)
}

func requesterMemoryFacts(t *testing.T, socketPath string) *http.Response {
	t.Helper()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		var dialer net.Dialer
		return dialer.DialContext(ctx, "unix", socketPath)
	}}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 3 * time.Second}
	request, errorValue := http.NewRequest(http.MethodGet, "http://internkim/memory/api/facts", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Header.Set(requesterEmailHeader, "member@example.com")
	response, errorValue := client.Do(request)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}

func memoryFactsClient(personaSeeded chan<- struct{}) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/admin/api/persona/agent" {
			if personaSeeded != nil {
				close(personaSeeded)
			}
			return jsonResponse(http.StatusServiceUnavailable, `{}`, nil), nil
		}
		if request.URL.String() == "https://app.example.test/api/agent/member" {
			return jsonResponse(http.StatusOK, `{"members":[{"email":"member@example.com","memberID":"user:person-1","name":"Member","role":"member","status":"active"}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/memory/facts" {
			return jsonResponse(http.StatusOK, `{"personID":"person-1","profile":{"identityLines":[],"currentLines":[]},"facts":[]}`, nil), nil
		}
		return nil, errors.New("unexpected memory startup request: " + request.URL.String())
	})}
}
