package capabilityd

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestMattermostWebSocketForwarderRunOnceReturnsWhenConnectionGoesIdle(t *testing.T) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer listener.Close()

	go serveIdleMattermostWebSocketHandshake(listener)

	forwarder := MattermostWebSocketForwarder{
		URL:             "ws://" + listener.Addr().String(),
		BotToken:        "test-bot-token",
		BlueclawURL:     "http://blueclaw.test",
		IdleReadTimeout: 50 * time.Millisecond,
	}

	resultChannel := make(chan error, 1)
	go func() {
		resultChannel <- forwarder.runOnce(context.Background())
	}()

	select {
	case errorValue := <-resultChannel:
		if errorValue == nil {
			t.Fatal("expected an idle read timeout error, got nil")
		}
		networkError, isNetworkError := errorValue.(net.Error)
		if !isNetworkError || !networkError.Timeout() {
			t.Fatalf("expected a network timeout error, got %v", errorValue)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runOnce did not return after the connection went idle; the websocket listener latched shut")
	}
}

func serveIdleMattermostWebSocketHandshake(listener net.Listener) {
	connection, errorValue := listener.Accept()
	if errorValue != nil {
		return
	}
	defer connection.Close()

	request, errorValue := http.ReadRequest(bufio.NewReader(connection))
	if errorValue != nil {
		return
	}
	defer request.Body.Close()

	secWebSocketKey := request.Header.Get("Sec-WebSocket-Key")
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + expectedWebSocketAccept(secWebSocketKey) + "\r\n\r\n"
	if _, errorValue := connection.Write([]byte(response)); errorValue != nil {
		return
	}

	time.Sleep(5 * time.Second)
}
