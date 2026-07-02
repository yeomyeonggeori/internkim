package browser

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const extensionBridgeRequestKeyMagic = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// ExtensionBridge is the daemon-side handle to the browser extension's
// background service worker connection. ExtensionInputRuntime depends on
// this interface rather than ExtensionWebSocketBridge directly so tests can
// inject a fake connection.
type ExtensionBridge interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Port() int
	WaitForReady(ctx context.Context) error
	RequestSnapshot(ctx context.Context) (ExtensionDOMSnapshot, error)
	ResolveRef(ctx context.Context, ref string) (ExtensionResolvedRef, error)
	Navigate(ctx context.Context, url string) (ExtensionDOMSnapshot, error)
}

// ExtensionWebSocketBridge listens on a local loopback port for a single
// browser extension connection and exchanges ExtensionBridgeMessage frames
// with it. It accepts the WebSocket upgrade by hand (RFC 6455) rather than
// pulling in a websocket dependency, mirroring the client-side frame codec
// already used for the Mattermost forwarder in internal/capabilityd.
type ExtensionWebSocketBridge struct {
	ListenAddress  string
	RequestTimeout time.Duration

	listener        net.Listener
	mutex           sync.Mutex
	connection      net.Conn
	readyChannel    chan struct{}
	readyClosed     atomic.Bool
	pendingRequests map[string]chan ExtensionBridgeMessage
	requestCounter  atomic.Uint64
}

func (bridge *ExtensionWebSocketBridge) Start(ctx context.Context) error {
	listener, errorValue := net.Listen("tcp", bridge.listenAddress())
	if errorValue != nil {
		return errorValue
	}
	bridge.listener = listener
	bridge.readyChannel = make(chan struct{})
	bridge.pendingRequests = map[string]chan ExtensionBridgeMessage{}
	go bridge.acceptLoop(ctx)
	return nil
}

func (bridge *ExtensionWebSocketBridge) Stop(ctx context.Context) error {
	_ = ctx
	bridge.mutex.Lock()
	connection := bridge.connection
	bridge.connection = nil
	bridge.mutex.Unlock()
	if connection != nil {
		_ = connection.Close()
	}
	if bridge.listener == nil {
		return nil
	}
	return bridge.listener.Close()
}

func (bridge *ExtensionWebSocketBridge) Port() int {
	if bridge.listener == nil {
		return 0
	}
	tcpAddress, ok := bridge.listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0
	}
	return tcpAddress.Port
}

func (bridge *ExtensionWebSocketBridge) WaitForReady(ctx context.Context) error {
	select {
	case <-bridge.readyChannel:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (bridge *ExtensionWebSocketBridge) RequestSnapshot(ctx context.Context) (ExtensionDOMSnapshot, error) {
	response, errorValue := bridge.roundTrip(ctx, ExtensionBridgeMessage{Type: ExtensionBridgeMessageRequestSnapshot})
	if errorValue != nil {
		return ExtensionDOMSnapshot{}, errorValue
	}
	if response.Snapshot == nil {
		return ExtensionDOMSnapshot{}, errors.New("browser extension returned no snapshot")
	}
	return *response.Snapshot, nil
}

func (bridge *ExtensionWebSocketBridge) ResolveRef(ctx context.Context, ref string) (ExtensionResolvedRef, error) {
	response, errorValue := bridge.roundTrip(ctx, ExtensionBridgeMessage{Type: ExtensionBridgeMessageResolveRef, Ref: ref})
	if errorValue != nil {
		return ExtensionResolvedRef{}, errorValue
	}
	if response.ResolvedRef == nil {
		return ExtensionResolvedRef{}, errors.New("browser extension returned no ref resolution")
	}
	return *response.ResolvedRef, nil
}

func (bridge *ExtensionWebSocketBridge) Navigate(ctx context.Context, url string) (ExtensionDOMSnapshot, error) {
	response, errorValue := bridge.roundTrip(ctx, ExtensionBridgeMessage{Type: ExtensionBridgeMessageNavigate, URL: url})
	if errorValue != nil {
		return ExtensionDOMSnapshot{}, errorValue
	}
	if response.Snapshot == nil {
		return ExtensionDOMSnapshot{}, errors.New("browser extension returned no snapshot after navigate")
	}
	return *response.Snapshot, nil
}

func (bridge *ExtensionWebSocketBridge) roundTrip(ctx context.Context, request ExtensionBridgeMessage) (ExtensionBridgeMessage, error) {
	requestContext, cancel := context.WithTimeout(ctx, bridge.requestTimeout())
	defer cancel()

	request.RequestID = bridge.nextRequestID()
	responseChannel := make(chan ExtensionBridgeMessage, 1)
	bridge.mutex.Lock()
	bridge.pendingRequests[request.RequestID] = responseChannel
	bridge.mutex.Unlock()
	defer func() {
		bridge.mutex.Lock()
		delete(bridge.pendingRequests, request.RequestID)
		bridge.mutex.Unlock()
	}()

	if errorValue := bridge.sendMessage(request); errorValue != nil {
		return ExtensionBridgeMessage{}, errorValue
	}

	select {
	case response := <-responseChannel:
		if response.Type == ExtensionBridgeMessageError {
			return ExtensionBridgeMessage{}, errors.New(firstNonEmpty(response.Error, "browser extension reported an error"))
		}
		return response, nil
	case <-requestContext.Done():
		return ExtensionBridgeMessage{}, requestContext.Err()
	}
}

func (bridge *ExtensionWebSocketBridge) requestTimeout() time.Duration {
	if bridge.RequestTimeout > 0 {
		return bridge.RequestTimeout
	}
	return 10 * time.Second
}

func (bridge *ExtensionWebSocketBridge) nextRequestID() string {
	return strconv.FormatUint(bridge.requestCounter.Add(1), 10)
}

func (bridge *ExtensionWebSocketBridge) listenAddress() string {
	if strings.TrimSpace(bridge.ListenAddress) != "" {
		return bridge.ListenAddress
	}
	return "127.0.0.1:0"
}

func (bridge *ExtensionWebSocketBridge) acceptLoop(ctx context.Context) {
	for {
		connection, errorValue := bridge.listener.Accept()
		if errorValue != nil {
			return
		}
		if errorValue := bridge.acceptConnection(ctx, connection); errorValue != nil {
			_ = connection.Close()
			continue
		}
	}
}

func (bridge *ExtensionWebSocketBridge) acceptConnection(ctx context.Context, connection net.Conn) error {
	reader := bufio.NewReader(connection)
	if errorValue := performExtensionBridgeHandshake(connection, reader); errorValue != nil {
		return errorValue
	}
	bridge.mutex.Lock()
	if bridge.connection != nil {
		_ = bridge.connection.Close()
	}
	bridge.connection = connection
	bridge.mutex.Unlock()
	go bridge.readLoop(ctx, connection, reader)
	return nil
}

func (bridge *ExtensionWebSocketBridge) readLoop(ctx context.Context, connection net.Conn, reader *bufio.Reader) {
	for ctx.Err() == nil {
		payload, errorValue := readExtensionBridgeFrame(connection, reader)
		if errorValue != nil {
			return
		}
		bridge.dispatchMessage(payload)
	}
}

func (bridge *ExtensionWebSocketBridge) dispatchMessage(payload []byte) {
	var message ExtensionBridgeMessage
	if json.Unmarshal(payload, &message) != nil {
		return
	}
	if message.Type == ExtensionBridgeMessageReady {
		bridge.markReady()
		return
	}
	if message.RequestID == "" {
		return
	}
	bridge.mutex.Lock()
	responseChannel, ok := bridge.pendingRequests[message.RequestID]
	bridge.mutex.Unlock()
	if !ok {
		return
	}
	responseChannel <- message
}

func (bridge *ExtensionWebSocketBridge) markReady() {
	if bridge.readyClosed.CompareAndSwap(false, true) {
		close(bridge.readyChannel)
	}
}

func (bridge *ExtensionWebSocketBridge) sendMessage(message ExtensionBridgeMessage) error {
	bridge.mutex.Lock()
	connection := bridge.connection
	bridge.mutex.Unlock()
	if connection == nil {
		return errors.New("browser extension bridge has no active connection")
	}
	document, errorValue := json.Marshal(message)
	if errorValue != nil {
		return errorValue
	}
	return writeExtensionBridgeFrame(connection, document)
}

func performExtensionBridgeHandshake(connection net.Conn, reader *bufio.Reader) error {
	request, errorValue := http.ReadRequest(reader)
	if errorValue != nil {
		return errorValue
	}
	defer request.Body.Close()
	requestKey := strings.TrimSpace(request.Header.Get("Sec-WebSocket-Key"))
	if !strings.EqualFold(request.Header.Get("Upgrade"), "websocket") || requestKey == "" {
		return errors.New("browser extension bridge received a non-websocket request")
	}
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + expectedWebSocketAcceptValue(requestKey) + "\r\n\r\n"
	_, errorValue = connection.Write([]byte(response))
	return errorValue
}

func expectedWebSocketAcceptValue(key string) string {
	sum := sha1.Sum([]byte(key + extensionBridgeRequestKeyMagic))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func readExtensionBridgeFrame(writer io.Writer, reader *bufio.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, errorValue := io.ReadFull(reader, header); errorValue != nil {
		return nil, errorValue
	}
	opcode := header[0] & 0x0f
	isMasked := header[1]&0x80 != 0
	payloadLength := uint64(header[1] & 0x7f)
	switch payloadLength {
	case 126:
		extendedLength := make([]byte, 2)
		if _, errorValue := io.ReadFull(reader, extendedLength); errorValue != nil {
			return nil, errorValue
		}
		payloadLength = uint64(binary.BigEndian.Uint16(extendedLength))
	case 127:
		extendedLength := make([]byte, 8)
		if _, errorValue := io.ReadFull(reader, extendedLength); errorValue != nil {
			return nil, errorValue
		}
		payloadLength = binary.BigEndian.Uint64(extendedLength)
	}

	var maskKey []byte
	if isMasked {
		maskKey = make([]byte, 4)
		if _, errorValue := io.ReadFull(reader, maskKey); errorValue != nil {
			return nil, errorValue
		}
	}

	payload := make([]byte, payloadLength)
	if _, errorValue := io.ReadFull(reader, payload); errorValue != nil {
		return nil, errorValue
	}
	if isMasked {
		for index := range payload {
			payload[index] ^= maskKey[index%4]
		}
	}

	switch opcode {
	case 0x1:
		return payload, nil
	case 0x8:
		return nil, errors.New("browser extension bridge connection closed")
	case 0x9:
		_ = writeExtensionBridgeControlFrame(writer, 0xA, payload)
		return readExtensionBridgeFrame(writer, reader)
	default:
		return readExtensionBridgeFrame(writer, reader)
	}
}

func writeExtensionBridgeFrame(writer io.Writer, payload []byte) error {
	return writeExtensionBridgeControlFrame(writer, 0x1, payload)
}

// writeExtensionBridgeControlFrame writes an unmasked server-to-client
// frame, per RFC 6455 section 5.1 (only client-to-server frames are masked).
func writeExtensionBridgeControlFrame(writer io.Writer, opcode byte, payload []byte) error {
	header := []byte{0x80 | opcode}
	payloadLength := len(payload)
	switch {
	case payloadLength < 126:
		header = append(header, byte(payloadLength))
	case payloadLength <= 65535:
		header = append(header, 126)
		lengthBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(lengthBytes, uint16(payloadLength))
		header = append(header, lengthBytes...)
	default:
		header = append(header, 127)
		lengthBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lengthBytes, uint64(payloadLength))
		header = append(header, lengthBytes...)
	}
	_, errorValue := writer.Write(append(header, payload...))
	return errorValue
}

func randomExtensionBridgeKey() (string, error) {
	value := make([]byte, 16)
	if _, errorValue := rand.Read(value); errorValue != nil {
		return "", errorValue
	}
	return base64.StdEncoding.EncodeToString(value), nil
}
