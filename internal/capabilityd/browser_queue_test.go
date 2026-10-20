package capabilityd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/workqueue"
)

func serviceWithAFullBrowser(t *testing.T, gateway http.HandlerFunc) Service {
	t.Helper()
	directory := t.TempDir()
	server := httptest.NewServer(gateway)
	t.Cleanup(server.Close)
	queuePath := filepath.Join(directory, "work-queue.json")
	sessionPath := filepath.Join(directory, "agent-key")
	if errorValue := os.WriteFile(sessionPath, []byte("host.session.token\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := workqueue.Write(queuePath, workqueue.Address{GatewayURL: server.URL, CompanyID: "company-1"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{
		Configuration:   Configuration{WorkQueuePath: queuePath, HostSessionPath: sessionPath},
		DeviceBrowsers:  oneBrowserInUseBySomeoneElse(t),
		handedElsewhere: &handedElsewhere{},
	}
}

func browserOpenFor(email string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: "browser_open",
		Input:    []byte(`{"url":"https://example.com"}`),
		Context:  capabilities.ToolInvokeContext{RequesterEmail: email},
	}
}

func oneBrowserInUseBySomeoneElse(t *testing.T) *browserruntime.DeviceBrowsers {
	t.Helper()
	stillLaunching := make(chan struct{})
	t.Cleanup(func() { close(stillLaunching) })
	browsers := browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: t.TempDir(),
		Capacity:       1,
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			<-stillLaunching
			return nil, context.Canceled
		},
		Now: time.Now,
	})
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	browsers.BrowserFor(cancelled, "someone-else@example.com")
	return browsers
}

func TestABrowserCallThisBoxHasNoRoomForIsAnsweredByTheBoxTheQueueGivesItTo(t *testing.T) {
	queued := 0
	service := serviceWithAFullBrowser(t, func(writer http.ResponseWriter, request *http.Request) {
		queued++
		if request.URL.Path != "/company/company-1/work" || request.Header.Get("Authorization") != "Bearer host.session.token" {
			t.Errorf("the queue was asked %s with %q", request.URL.Path, request.Header.Get("Authorization"))
		}
		writer.Write([]byte(`{"requestID":"x","status":200,"body":{"toolName":"browser_open","status":"ok","content":"opened on another box"}}`))
	})

	for range 2 {
		response, errorValue := service.invokeDeviceBrowserTool(context.Background(), browserOpenFor("Sample@Example.com"))
		if errorValue != nil || response.Content != "opened on another box" {
			t.Fatalf("answered %+v, %v", response, errorValue)
		}
	}
	if queued != 2 || !service.handedElsewhere.has("sample@example.com") {
		t.Fatalf("queued %d calls; a person whose browser runs elsewhere keeps going there", queued)
	}
}

func TestABrowserCallNoBoxHasRoomForIsAnsweredThatTheBrowserIsBusy(t *testing.T) {
	service := serviceWithAFullBrowser(t, func(writer http.ResponseWriter, _ *http.Request) {
		writer.Write([]byte(`{"requestID":"x","status":503,"body":{"error":"no box had room for this work"}}`))
	})

	response, errorValue := service.invokeDeviceBrowserTool(context.Background(), browserOpenFor("sample@example.com"))

	if errorValue != nil || !response.IsError || !strings.Contains(response.Content, "device browsers are in use") {
		t.Fatalf("answered %+v, %v", response, errorValue)
	}
}

func TestABoxWithoutAQueueAddressAnswersBusyWithoutAskingTheGateway(t *testing.T) {
	asked := 0
	service := serviceWithAFullBrowser(t, func(http.ResponseWriter, *http.Request) { asked++ })
	if errorValue := workqueue.Forget(service.Configuration.WorkQueuePath); errorValue != nil {
		t.Fatal(errorValue)
	}

	response, errorValue := service.invokeDeviceBrowserTool(context.Background(), browserOpenFor("sample@example.com"))

	if errorValue != nil || !response.IsError || asked != 0 {
		t.Fatalf("answered %+v, %v after asking the gateway %d times", response, errorValue, asked)
	}
}
