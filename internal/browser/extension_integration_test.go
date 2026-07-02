package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// serveExtensionRequests plays the role of the browser extension's
// background service worker, answering every request frame the bridge sends
// over the real WebSocket connection with a canned response, exactly like
// companion/browser-extension/background.js answers requestSnapshot,
// resolveRef, and navigate after asking content.js for DOM data. It runs
// until the connection closes, so it must be started in a goroutine.
func serveExtensionRequests(t *testing.T, connection net.Conn, reader *bufio.Reader, snapshot ExtensionDOMSnapshot, resolvedRefByRef map[string]ExtensionResolvedRef) {
	t.Helper()
	for {
		payload, errorValue := readExtensionBridgeFrame(connection, reader)
		if errorValue != nil {
			return
		}
		var request ExtensionBridgeMessage
		if json.Unmarshal(payload, &request) != nil {
			continue
		}
		response := extensionClientResponseFor(request, snapshot, resolvedRefByRef)
		document, marshalError := json.Marshal(response)
		if marshalError != nil {
			t.Errorf("failed to marshal fake extension response: %v", marshalError)
			return
		}
		if writeError := writeExtensionBridgeFrame(connection, document); writeError != nil {
			return
		}
	}
}

func extensionClientResponseFor(request ExtensionBridgeMessage, snapshot ExtensionDOMSnapshot, resolvedRefByRef map[string]ExtensionResolvedRef) ExtensionBridgeMessage {
	switch request.Type {
	case ExtensionBridgeMessageRequestSnapshot:
		return ExtensionBridgeMessage{Type: ExtensionBridgeMessageDOMSnapshot, RequestID: request.RequestID, Snapshot: &snapshot}
	case ExtensionBridgeMessageNavigate:
		navigatedSnapshot := snapshot
		navigatedSnapshot.URL = request.URL
		return ExtensionBridgeMessage{Type: ExtensionBridgeMessageNavigated, RequestID: request.RequestID, Snapshot: &navigatedSnapshot}
	case ExtensionBridgeMessageResolveRef:
		resolved, found := resolvedRefByRef[request.Ref]
		if !found {
			resolved = ExtensionResolvedRef{Ref: request.Ref, Found: false}
		}
		return ExtensionBridgeMessage{Type: ExtensionBridgeMessageResolvedRef, RequestID: request.RequestID, ResolvedRef: &resolved}
	default:
		return ExtensionBridgeMessage{Type: ExtensionBridgeMessageError, RequestID: request.RequestID, Error: "fake extension client does not handle " + string(request.Type)}
	}
}

// TestExtensionInputRuntimeEndToEndOverRealBridge drives a full
// StartSession -> Observe -> Click -> Pause -> Resume -> Click sequence
// through the real ExtensionWebSocketBridge (actual TCP + RFC 6455 frames,
// not a mocked ExtensionBridge) against a fake client that plays the
// extension's part, with a fake OSInputSynthesizer standing in for the
// platform-specific input code. This is the strongest coverage possible
// without a real Chrome and browser extension.
func TestExtensionInputRuntimeEndToEndOverRealBridge(t *testing.T) {
	extensionDirectory := t.TempDir()
	bridge := &ExtensionWebSocketBridge{ListenAddress: "127.0.0.1:0", RequestTimeout: 5 * time.Second}
	synthesizer := &fakeInputSynthesizer{}
	runtime := &ExtensionInputRuntime{
		Bridge:           bridge,
		ExtensionPath:    extensionDirectory,
		Runner:           &fakeCommandRunner{},
		InputSynthesizer: synthesizer,
		ReadyTimeout:     5 * time.Second,
	}

	snapshot := ExtensionDOMSnapshot{
		Title: "Example Page",
		Elements: []ExtensionElementDescriptor{
			{Ref: "e1", Tag: "button", Text: "Continue"},
		},
	}
	resolvedRefs := map[string]ExtensionResolvedRef{
		"@e1": {
			Ref:   "@e1",
			Found: true,
			Rect:  ExtensionElementRect{X: 100, Y: 60, Width: 20, Height: 10},
			Viewport: ExtensionViewportGeometry{
				ScreenX: 0, ScreenY: 0, OuterWidth: 100, InnerWidth: 100, OuterHeight: 100, InnerHeight: 100,
			},
		},
	}

	startResultChannel := make(chan SessionStartResult, 1)
	startErrorChannel := make(chan error, 1)
	go func() {
		result, errorValue := runtime.StartSession(context.Background(), SessionStartRequest{URL: "https://example.com/login"})
		startResultChannel <- result
		startErrorChannel <- errorValue
	}()

	// waitForRuntimeConfigPort synchronizes on the runtime-config.json file
	// StartSession writes right after Start(), rather than polling
	// bridge.Port() directly: reading that in-memory field from this
	// goroutine while StartSession's goroutine is still inside Start() would
	// be a genuine data race on the bridge's unexported listener field, even
	// though production code never calls Port() concurrently with Start().
	port := waitForRuntimeConfigPort(t, extensionDirectory)
	connection, frameReader := dialExtensionBridgeTestClient(t, "127.0.0.1:"+strconv.Itoa(port))
	defer connection.Close()

	go serveExtensionRequests(t, connection, frameReader, snapshot, resolvedRefs)
	sendReadyMessage(t, connection)

	startResult := <-startResultChannel
	if startError := <-startErrorChannel; startError != nil {
		t.Fatalf("expected StartSession to succeed over the real bridge: %v", startError)
	}
	if !startResult.Opened || startResult.Title != "Example Page" {
		t.Fatalf("unexpected start session result: %+v", startResult)
	}
	if !containsString(startResult.InteractiveRefs, "@e1") {
		t.Fatalf("expected the fake extension's snapshot ref in start result: %+v", startResult.InteractiveRefs)
	}

	observeResult, observeError := runtime.Observe(context.Background(), ObserveRequest{})
	if observeError != nil {
		t.Fatalf("expected observe to succeed over the real bridge: %v", observeError)
	}
	if observeResult.Title != "Example Page" {
		t.Fatalf("unexpected observe result: %+v", observeResult)
	}

	clickResult, clickError := runtime.Click(context.Background(), ClickRequest{Ref: "@e1"})
	if clickError != nil {
		t.Fatalf("expected click to succeed over the real bridge: %v", clickError)
	}
	if !clickResult.OK {
		t.Fatalf("unexpected click result: %+v", clickResult)
	}
	if len(synthesizer.calls) != 1 || synthesizer.calls[0].screenX != 110 || synthesizer.calls[0].screenY != 65 {
		t.Fatalf("expected a synthesized click at the resolved screen point, got %+v", synthesizer.calls)
	}

	if pauseError := runtime.Pause(context.Background()); pauseError != nil {
		t.Fatalf("expected pause to succeed: %v", pauseError)
	}
	if _, clickError := runtime.Click(context.Background(), ClickRequest{Ref: "@e1"}); clickError == nil {
		t.Fatal("expected click to fail while the runtime is paused")
	}
	if len(synthesizer.calls) != 1 {
		t.Fatalf("expected no additional synthesizer calls while paused, got %+v", synthesizer.calls)
	}

	if resumeError := runtime.Resume(context.Background()); resumeError != nil {
		t.Fatalf("expected resume to succeed: %v", resumeError)
	}
	if _, clickError := runtime.Click(context.Background(), ClickRequest{Ref: "@e1"}); clickError != nil {
		t.Fatalf("expected click to succeed after resume: %v", clickError)
	}
	if len(synthesizer.calls) != 2 {
		t.Fatalf("expected a second synthesizer call after resume, got %+v", synthesizer.calls)
	}
}

func waitForRuntimeConfigPort(t *testing.T, extensionDirectory string) int {
	t.Helper()
	configurationPath := filepath.Join(extensionDirectory, "runtime-config.json")
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		document, readError := os.ReadFile(configurationPath)
		if readError == nil {
			var configuration struct {
				Port int `json:"port"`
			}
			if json.Unmarshal(document, &configuration) == nil && configuration.Port != 0 {
				return configuration.Port
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for runtime-config.json to report the bridge port")
	return 0
}

func sendReadyMessage(t *testing.T, connection net.Conn) {
	t.Helper()
	document, errorValue := json.Marshal(ExtensionBridgeMessage{
		Type:  ExtensionBridgeMessageReady,
		Ready: &ExtensionReadyPayload{ExtensionVersion: "1.0.0", TabID: 1},
	})
	if errorValue != nil {
		t.Fatalf("expected to marshal the ready message: %v", errorValue)
	}
	if errorValue := writeExtensionBridgeFrame(connection, document); errorValue != nil {
		t.Fatalf("expected to write the ready frame: %v", errorValue)
	}
}
