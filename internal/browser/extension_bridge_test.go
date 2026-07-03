package browser

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func dialExtensionBridgeTestClient(t *testing.T, address string) (net.Conn, *bufio.Reader) {
	t.Helper()
	connection, errorValue := net.Dial("tcp", address)
	if errorValue != nil {
		t.Fatalf("expected to dial the bridge listener: %v", errorValue)
	}
	secWebSocketKey, errorValue := randomExtensionBridgeKey()
	if errorValue != nil {
		t.Fatalf("expected to generate a websocket key: %v", errorValue)
	}
	request := "GET / HTTP/1.1\r\n" +
		"Host: " + address + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + secWebSocketKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if _, errorValue := connection.Write([]byte(request)); errorValue != nil {
		t.Fatalf("expected to write the upgrade request: %v", errorValue)
	}
	reader := bufio.NewReader(connection)
	response, errorValue := http.ReadResponse(reader, nil)
	if errorValue != nil {
		t.Fatalf("expected an upgrade response: %v", errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101 switching protocols, got %s", response.Status)
	}
	if response.Header.Get("Sec-WebSocket-Accept") != expectedWebSocketAcceptValue(secWebSocketKey) {
		t.Fatal("expected a matching Sec-WebSocket-Accept header")
	}
	return connection, reader
}

func TestExtensionWebSocketBridgeRoundTripsResolveRef(t *testing.T) {
	bridge := &ExtensionWebSocketBridge{ListenAddress: "127.0.0.1:0", RequestTimeout: 2 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if errorValue := bridge.Start(ctx); errorValue != nil {
		t.Fatalf("expected bridge to start: %v", errorValue)
	}
	defer bridge.Stop(context.Background())

	connection, reader := dialExtensionBridgeTestClient(t, "127.0.0.1:"+strconv.Itoa(bridge.Port()))
	defer connection.Close()

	readyDocument, errorValue := json.Marshal(ExtensionBridgeMessage{Type: ExtensionBridgeMessageReady})
	if errorValue != nil {
		t.Fatalf("expected to marshal the ready message: %v", errorValue)
	}
	if errorValue := writeExtensionBridgeFrame(connection, readyDocument); errorValue != nil {
		t.Fatalf("expected to write the ready frame: %v", errorValue)
	}

	waitReadyContext, cancelWaitReady := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelWaitReady()
	if errorValue := bridge.WaitForReady(waitReadyContext); errorValue != nil {
		t.Fatalf("expected the bridge to observe readiness: %v", errorValue)
	}

	responseChannel := make(chan ExtensionResolvedRef, 1)
	errorChannel := make(chan error, 1)
	go func() {
		resolved, errorValue := bridge.ResolveRef(context.Background(), "@e1")
		if errorValue != nil {
			errorChannel <- errorValue
			return
		}
		responseChannel <- resolved
	}()

	requestPayload, errorValue := readExtensionBridgeFrame(connection, reader)
	if errorValue != nil {
		t.Fatalf("expected to read the resolveRef request frame: %v", errorValue)
	}
	var request ExtensionBridgeMessage
	if errorValue := json.Unmarshal(requestPayload, &request); errorValue != nil {
		t.Fatalf("expected a valid resolveRef request: %v", errorValue)
	}
	if request.Type != ExtensionBridgeMessageResolveRef || request.Ref != "@e1" {
		t.Fatalf("unexpected resolveRef request: %+v", request)
	}

	responseDocument, errorValue := json.Marshal(ExtensionBridgeMessage{
		Type:      ExtensionBridgeMessageResolvedRef,
		RequestID: request.RequestID,
		ResolvedRef: &ExtensionResolvedRef{
			Ref:   "@e1",
			Found: true,
			Rect:  ExtensionElementRect{X: 1, Y: 2, Width: 3, Height: 4},
		},
	})
	if errorValue != nil {
		t.Fatalf("expected to marshal the resolvedRef response: %v", errorValue)
	}
	if errorValue := writeExtensionBridgeFrame(connection, responseDocument); errorValue != nil {
		t.Fatalf("expected to write the resolvedRef response frame: %v", errorValue)
	}

	select {
	case resolved := <-responseChannel:
		if !resolved.Found || resolved.Rect.Width != 3 {
			t.Fatalf("unexpected resolved ref: %+v", resolved)
		}
	case errorValue := <-errorChannel:
		t.Fatalf("expected resolve ref to succeed: %v", errorValue)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for resolve ref to complete")
	}
}
