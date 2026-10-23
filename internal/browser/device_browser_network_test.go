package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestADeviceBrowserPageCannotReachTheHostsOwnNetwork(t *testing.T) {
	executablePath := os.Getenv("INTERNKIM_TEST_MOLI")
	if executablePath == "" {
		t.Skip("INTERNKIM_TEST_MOLI names a moli binary to start")
	}
	hostService := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Access-Control-Allow-Origin", "*")
		_, _ = writer.Write([]byte("reached"))
	}))
	t.Cleanup(hostService.Close)
	requesterBrowserPort := launchTestDeviceBrowser(t, executablePath, "requester")
	otherPersonsBrowserPort := launchTestDeviceBrowser(t, executablePath, "other-person")
	otherPersonsDevtools, errorValue := devtoolsBrowserSocketURL(context.Background(), fmt.Sprintf("http://127.0.0.1:%d", otherPersonsBrowserPort))
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	cases := map[string]string{
		"another person's browser": fmt.Sprintf(`new Promise(resolve => {
			const socket = new WebSocket(%q);
			socket.onopen = () => resolve("reached");
			socket.onerror = () => resolve("blocked");
			setTimeout(() => resolve("blocked"), 3000);
		})`, otherPersonsDevtools),
		"a service on the host": fmt.Sprintf(`fetch(%q).then(response => response.text()).catch(() => "blocked")`, hostService.URL),
	}
	for name, script := range cases {
		t.Run(name, func(t *testing.T) {
			if outcome := evaluateInDeviceBrowserPage(t, requesterBrowserPort, script); outcome != "blocked" {
				t.Fatalf("a page in the requester's device browser reached %s: %q", name, outcome)
			}
		})
	}
}

func launchTestDeviceBrowser(t *testing.T, executablePath string, memberKey string) int {
	t.Helper()
	port := freeTestPort(t)
	memberDirectory := filepath.Join(t.TempDir(), "members", memberKey)
	running, errorValue := LaunchDeviceBrowserProcess(context.Background(), DeviceBrowserLaunch{
		ExecutablePath:   executablePath,
		Port:             port,
		MemberDirectory:  memberDirectory,
		ProfileDirectory: filepath.Join(memberDirectory, "profile"),
		CacheDirectory:   filepath.Join(memberDirectory, "cache"),
		LogPath:          filepath.Join(memberDirectory, "moli.log"),
	})
	if errorValue != nil {
		t.Fatalf("expected moli to start: %v", errorValue)
	}
	t.Cleanup(running.Stop)
	return port
}

func freeTestPort(t *testing.T) int {
	t.Helper()
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func evaluateInDeviceBrowserPage(t *testing.T, port int, script string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	socketURL, errorValue := devtoolsBrowserSocketURL(ctx, fmt.Sprintf("http://127.0.0.1:%d", port))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	connection, errorValue := dialDevtoolsSocket(ctx, socketURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer connection.close()
	sessionID := attachToFirstPage(t, ctx, connection)
	evaluated, errorValue := connection.call(ctx, sessionID, "Runtime.evaluate", map[string]any{"expression": script, "awaitPromise": true, "returnByValue": true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var outcome struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	if errorValue := json.Unmarshal(evaluated, &outcome); errorValue != nil {
		t.Fatal(errorValue)
	}
	return outcome.Result.Value
}

func attachToFirstPage(t *testing.T, ctx context.Context, connection *devtoolsConnection) string {
	t.Helper()
	targets, errorValue := connection.call(ctx, "", "Target.getTargets", map[string]any{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var targetList struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
		} `json:"targetInfos"`
	}
	if errorValue := json.Unmarshal(targets, &targetList); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, target := range targetList.TargetInfos {
		if target.Type != "page" {
			continue
		}
		attached, errorValue := connection.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true})
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		var session struct {
			SessionID string `json:"sessionId"`
		}
		if errorValue := json.Unmarshal(attached, &session); errorValue != nil {
			t.Fatal(errorValue)
		}
		return session.SessionID
	}
	t.Fatal("the device browser has no page")
	return ""
}
