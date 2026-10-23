package capabilityd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	browserruntime "github.com/yeomyeonggeori/internkim/internal/browser"
)

type fakeRunningBrowser struct {
	exited   chan struct{}
	stopOnce sync.Once
}

func (browser *fakeRunningBrowser) Exited() <-chan struct{} {
	return browser.exited
}

func (browser *fakeRunningBrowser) Stop() {
	browser.stopOnce.Do(func() { close(browser.exited) })
}

func fakeDeviceBrowsers(capacity int, now time.Time) *browserruntime.DeviceBrowsers {
	return browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: "/var/lib/internkim/device-browsers",
		FirstPort:      browserruntime.DeviceBrowsersFirstPort,
		Capacity:       capacity,
		Now:            func() time.Time { return now },
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			return &fakeRunningBrowser{exited: make(chan struct{})}, nil
		},
	})
}

func deviceBrowsersWhoseLaunchWaits(capacity int) (*browserruntime.DeviceBrowsers, <-chan struct{}, func()) {
	launchStarted := make(chan struct{}, capacity)
	launchGate := make(chan struct{})
	browsers := browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: "/var/lib/internkim/device-browsers",
		FirstPort:      browserruntime.DeviceBrowsersFirstPort,
		Capacity:       capacity,
		Now:            time.Now,
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			launchStarted <- struct{}{}
			<-launchGate
			return &fakeRunningBrowser{exited: make(chan struct{})}, nil
		},
	})
	return browsers, launchStarted, func() { close(launchGate) }
}

func fakeDeviceBrowsersWithEmptyDevtools(t *testing.T) *browserruntime.DeviceBrowsers {
	return browserruntime.NewDeviceBrowsers(browserruntime.DeviceBrowserSettings{
		StateDirectory: "/var/lib/internkim/device-browsers",
		FirstPort:      devtoolsHoldingNoPages(t),
		Capacity:       1,
		Launch: func(context.Context, browserruntime.DeviceBrowserLaunch) (browserruntime.RunningDeviceBrowser, error) {
			return &fakeRunningBrowser{exited: make(chan struct{})}, nil
		},
	})
}

func devtoolsHoldingNoPages(t *testing.T) int {
	t.Helper()
	multiplexer := http.NewServeMux()
	server := httptest.NewServer(multiplexer)
	t.Cleanup(server.Close)
	multiplexer.HandleFunc("/json/version", func(writer http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(writer).Encode(map[string]string{"webSocketDebuggerUrl": "ws" + strings.TrimPrefix(server.URL, "http") + "/devtools/browser/fake"})
	})
	multiplexer.HandleFunc("/devtools/browser/fake", answerDevtoolsWithNoPages)
	return server.Listener.Addr().(*net.TCPAddr).Port
}

func answerDevtoolsWithNoPages(writer http.ResponseWriter, request *http.Request) {
	socket, errorValue := websocket.Accept(writer, request, nil)
	if errorValue != nil {
		return
	}
	defer socket.CloseNow()
	for {
		_, document, errorValue := socket.Read(request.Context())
		if errorValue != nil {
			return
		}
		var command struct {
			ID     int64  `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(document, &command) != nil {
			continue
		}
		result := map[string]any{}
		if command.Method == "Target.getTargets" {
			result["targetInfos"] = []any{}
		}
		answer, _ := json.Marshal(map[string]any{"id": command.ID, "result": result})
		if socket.Write(request.Context(), websocket.MessageText, answer) != nil {
			return
		}
	}
}

type guardHoldingNothing struct{}

func (guardHoldingNothing) IsGuarding(context.Context, string) bool { return true }

func (guardHoldingNothing) Settle(context.Context) {}

func (guardHoldingNothing) NotePasswordInputs(context.Context) {}

func (guardHoldingNothing) TakeWithheld() browserruntime.GuardedRequests {
	return browserruntime.GuardedRequests{}
}

func (guardHoldingNothing) Resend(context.Context, browserruntime.WithheldRequest, string, func() error) error {
	return nil
}

func (guardHoldingNothing) Close() {}

func guardsHoldingNothing() *browserRequestGuards {
	return &browserRequestGuards{attach: func(context.Context, string) (pageRequestGuard, error) {
		return guardHoldingNothing{}, nil
	}}
}
