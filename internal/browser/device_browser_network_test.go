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

	"github.com/coder/websocket"
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
	otherPersonsDevtools := devtoolsBrowserSocketURL(t, otherPersonsBrowserPort)

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

func devtoolsBrowserSocketURL(t *testing.T, port int) string {
	t.Helper()
	response, errorValue := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer response.Body.Close()
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&version); errorValue != nil {
		t.Fatal(errorValue)
	}
	return version.WebSocketDebuggerURL
}

func evaluateInDeviceBrowserPage(t *testing.T, port int, script string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	socket, _, errorValue := websocket.Dial(ctx, devtoolsBrowserSocketURL(t, port), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer socket.CloseNow()
	targets := sendTestDevtoolsCommand(t, ctx, socket, "", "Target.getTargets", map[string]any{})
	var targetList struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
		} `json:"targetInfos"`
	}
	json.Unmarshal(targets, &targetList)
	pageTargetID := ""
	for _, target := range targetList.TargetInfos {
		if target.Type == "page" {
			pageTargetID = target.TargetID
		}
	}
	attached := sendTestDevtoolsCommand(t, ctx, socket, "", "Target.attachToTarget", map[string]any{"targetId": pageTargetID, "flatten": true})
	var session struct {
		SessionID string `json:"sessionId"`
	}
	json.Unmarshal(attached, &session)
	evaluated := sendTestDevtoolsCommand(t, ctx, socket, session.SessionID, "Runtime.evaluate", map[string]any{"expression": script, "awaitPromise": true, "returnByValue": true})
	var outcome struct {
		Result struct {
			Value string `json:"value"`
		} `json:"result"`
	}
	json.Unmarshal(evaluated, &outcome)
	return outcome.Result.Value
}

var testDevtoolsCommandID int

func sendTestDevtoolsCommand(t *testing.T, ctx context.Context, socket *websocket.Conn, sessionID string, method string, parameters map[string]any) json.RawMessage {
	t.Helper()
	testDevtoolsCommandID++
	commandID := testDevtoolsCommandID
	command := map[string]any{"id": commandID, "method": method, "params": parameters}
	if sessionID != "" {
		command["sessionId"] = sessionID
	}
	document, _ := json.Marshal(command)
	if errorValue := socket.Write(ctx, websocket.MessageText, document); errorValue != nil {
		t.Fatal(errorValue)
	}
	for {
		_, message, errorValue := socket.Read(ctx)
		if errorValue != nil {
			t.Fatalf("%s: %v", method, errorValue)
		}
		var answer struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		json.Unmarshal(message, &answer)
		if answer.ID != commandID {
			continue
		}
		if len(answer.Error) > 0 {
			t.Fatalf("%s: %s", method, answer.Error)
		}
		return answer.Result
	}
}
