package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/coder/websocket"
)

const devtoolsMessageLimit = 64 << 20

type devtoolsMessage struct {
	ID        int64           `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     *devtoolsError  `json:"error,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
}

type devtoolsError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type devtoolsConnection struct {
	socket   *websocket.Conn
	mutex    sync.Mutex
	nextID   int64
	answers  map[int64]chan devtoolsMessage
	events   chan devtoolsMessage
	received atomic.Int64
	closed   chan struct{}
	readDone chan struct{}
}

func dialDevtools(ctx context.Context, devtoolsURL string) (*devtoolsConnection, error) {
	socketURL, errorValue := devtoolsBrowserSocketURL(ctx, devtoolsURL)
	if errorValue != nil {
		return nil, errorValue
	}
	socket, _, errorValue := websocket.Dial(ctx, socketURL, nil)
	if errorValue != nil {
		return nil, fmt.Errorf("the device browser's DevTools socket could not be opened: %w", errorValue)
	}
	socket.SetReadLimit(devtoolsMessageLimit)
	connection := &devtoolsConnection{
		socket:   socket,
		answers:  map[int64]chan devtoolsMessage{},
		events:   make(chan devtoolsMessage, 256),
		closed:   make(chan struct{}),
		readDone: make(chan struct{}),
	}
	go connection.read()
	return connection, nil
}

func devtoolsBrowserSocketURL(ctx context.Context, devtoolsURL string) (string, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(devtoolsURL, "/")+"/json/version", nil)
	if errorValue != nil {
		return "", errorValue
	}
	response, errorValue := deviceBrowserHTTPClient.Do(request)
	if errorValue != nil {
		return "", fmt.Errorf("the device browser's DevTools endpoint did not answer: %w", errorValue)
	}
	defer response.Body.Close()
	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&version); errorValue != nil || version.WebSocketDebuggerURL == "" {
		return "", errors.New("the device browser's DevTools endpoint named no browser socket")
	}
	return version.WebSocketDebuggerURL, nil
}

func (connection *devtoolsConnection) read() {
	defer close(connection.readDone)
	defer close(connection.events)
	for {
		_, document, errorValue := connection.socket.Read(context.Background())
		if errorValue != nil {
			connection.failWaitingCalls()
			return
		}
		var message devtoolsMessage
		if json.Unmarshal(document, &message) != nil {
			continue
		}
		if message.ID != 0 {
			connection.deliverAnswer(message)
			continue
		}
		connection.received.Add(1)
		select {
		case connection.events <- message:
		case <-connection.closed:
			return
		}
	}
}

func (connection *devtoolsConnection) deliverAnswer(message devtoolsMessage) {
	connection.mutex.Lock()
	answer, isWaiting := connection.answers[message.ID]
	delete(connection.answers, message.ID)
	connection.mutex.Unlock()
	if isWaiting {
		answer <- message
	}
}

func (connection *devtoolsConnection) failWaitingCalls() {
	connection.mutex.Lock()
	defer connection.mutex.Unlock()
	for identifier, answer := range connection.answers {
		answer <- devtoolsMessage{ID: identifier, Error: &devtoolsError{Message: "the DevTools connection closed"}}
		delete(connection.answers, identifier)
	}
}

func (connection *devtoolsConnection) call(ctx context.Context, sessionID string, method string, parameters any) (json.RawMessage, error) {
	encodedParameters, errorValue := json.Marshal(parameters)
	if errorValue != nil {
		return nil, errorValue
	}
	answer := make(chan devtoolsMessage, 1)
	connection.mutex.Lock()
	connection.nextID++
	identifier := connection.nextID
	connection.answers[identifier] = answer
	connection.mutex.Unlock()
	document, errorValue := json.Marshal(devtoolsMessage{ID: identifier, Method: method, Params: encodedParameters, SessionID: sessionID})
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := connection.socket.Write(ctx, websocket.MessageText, document); errorValue != nil {
		return nil, fmt.Errorf("%s could not be sent to the device browser: %w", method, errorValue)
	}
	select {
	case message := <-answer:
		if message.Error != nil {
			return nil, fmt.Errorf("the device browser refused %s: %s", method, message.Error.Message)
		}
		return message.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (connection *devtoolsConnection) close() {
	close(connection.closed)
	_ = connection.socket.CloseNow()
	<-connection.readDone
}
