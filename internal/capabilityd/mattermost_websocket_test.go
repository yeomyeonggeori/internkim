package capabilityd

import (
	"bufio"
	"context"
	"errors"
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
		_, errorValue := forwarder.runOnce(context.Background())
		resultChannel <- errorValue
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

func TestMattermostWebSocketForwarderKeepsIdleConnectionAliveWithPing(t *testing.T) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer listener.Close()

	pingChannel := make(chan struct{}, 1)
	releaseConnection := make(chan struct{})
	defer close(releaseConnection)
	go serveMattermostWebSocketPong(listener, pingChannel, releaseConnection)

	forwarder := MattermostWebSocketForwarder{
		URL:             "ws://" + listener.Addr().String(),
		BotToken:        "test-bot-token",
		BlueclawURL:     "http://blueclaw.test",
		IdleReadTimeout: 100 * time.Millisecond,
	}
	ctx, cancel := context.WithCancel(context.Background())
	resultChannel := make(chan error, 1)
	go func() {
		_, errorValue := forwarder.runOnce(ctx)
		resultChannel <- errorValue
	}()

	select {
	case <-pingChannel:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("forwarder did not send an idle keepalive ping")
	}

	select {
	case errorValue := <-resultChannel:
		if !errors.Is(errorValue, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", errorValue)
		}
	case <-time.After(time.Second):
		t.Fatal("forwarder did not stop after context cancellation")
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

func serveMattermostWebSocketPong(listener net.Listener, pingChannel chan<- struct{}, releaseConnection <-chan struct{}) {
	connection, errorValue := listener.Accept()
	if errorValue != nil {
		return
	}
	defer connection.Close()

	reader := bufio.NewReader(connection)
	request, errorValue := http.ReadRequest(reader)
	if errorValue != nil {
		return
	}
	defer request.Body.Close()

	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + expectedWebSocketAccept(request.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n"
	if _, errorValue := connection.Write([]byte(response)); errorValue != nil {
		return
	}

	if _, errorValue := readWebSocketFrame(connection, reader); errorValue != nil {
		return
	}
	if _, errorValue := readWebSocketFrame(connection, reader); errorValue != nil {
		return
	}
	pingChannel <- struct{}{}
	<-releaseConnection
}
