package capabilityd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha1"
	"crypto/tls"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultMattermostWebSocketIdleReadTimeout = 90 * time.Second
const defaultMattermostWebSocketHandshakeTimeout = 15 * time.Second

type MattermostWebSocketForwarder struct {
	URL               string
	BotToken          string
	BotUserID         string
	BotUsername       string
	BlueclawURL       string
	HTTPClient        *http.Client
	IdleReadTimeout   time.Duration
	EventBuilder      func(context.Context, []byte) (platformInboundEvent, bool, error)
	AfterForward      func(context.Context, []byte)
	AfterForwardError func(error)
	PollFallback      func(context.Context)
}

func (forwarder MattermostWebSocketForwarder) Start(ctx context.Context) {
	if strings.TrimSpace(forwarder.URL) == "" || strings.TrimSpace(forwarder.BotToken) == "" || strings.TrimSpace(forwarder.BlueclawURL) == "" {
		log.Print("mattermost forwarder disabled")
		return
	}

	backoff := time.Second
	for ctx.Err() == nil {
		wasConnected, errorValue := forwarder.runOnce(ctx)
		if wasConnected {
			backoff = time.Second
		}
		if errorValue != nil && ctx.Err() == nil {
			log.Printf("mattermost forwarder disconnected: botUserID=%s botUsername=%s error=%v", forwarder.BotUserID, forwarder.BotUsername, errorValue)
			if forwarder.PollFallback != nil {
				forwarder.PollFallback(ctx)
			}
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		log.Printf("mattermost forwarder reconnecting: botUserID=%s botUsername=%s backoff=%s", forwarder.BotUserID, forwarder.BotUsername, backoff)
		if !wasConnected && backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (forwarder MattermostWebSocketForwarder) idleReadTimeout() time.Duration {
	if forwarder.IdleReadTimeout > 0 {
		return forwarder.IdleReadTimeout
	}
	return defaultMattermostWebSocketIdleReadTimeout
}

func (forwarder MattermostWebSocketForwarder) runOnce(ctx context.Context) (bool, error) {
	connection, reader, errorValue := forwarder.connect(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer connection.Close()

	if errorValue := writeWebSocketTextFrame(connection, forwarder.authenticationChallenge()); errorValue != nil {
		return false, errorValue
	}
	log.Printf("mattermost forwarder connected: botUserID=%s botUsername=%s", forwarder.BotUserID, forwarder.BotUsername)

	isAwaitingPong := false
	for ctx.Err() == nil {
		if errorValue := connection.SetReadDeadline(time.Now().Add(forwarder.idleReadTimeout() / 2)); errorValue != nil {
			return true, errorValue
		}
		payload, errorValue := readWebSocketFrame(connection, reader)
		if errorValue != nil {
			if !isTimeout(errorValue) || isAwaitingPong {
				return true, errorValue
			}
			if errorValue := writeWebSocketFrame(connection, 0x9, nil); errorValue != nil {
				return true, errorValue
			}
			isAwaitingPong = true
			continue
		}
		isAwaitingPong = false
		if len(payload) == 0 {
			continue
		}
		payloadDocument := append([]byte(nil), payload...)
		go func() {
			if errorValue := forwarder.forward(ctx, payloadDocument); errorValue != nil {
				if forwarder.AfterForwardError != nil {
					forwarder.AfterForwardError(errorValue)
				}
				log.Printf("mattermost forward failed: botUserID=%s botUsername=%s error=%v", forwarder.BotUserID, forwarder.BotUsername, errorValue)
			}
		}()
	}
	return true, ctx.Err()
}

func isTimeout(errorValue error) bool {
	networkError, isNetworkError := errorValue.(net.Error)
	return isNetworkError && networkError.Timeout()
}

func (forwarder MattermostWebSocketForwarder) connect(ctx context.Context) (net.Conn, *bufio.Reader, error) {
	parsedURL, errorValue := url.Parse(forwarder.URL)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if parsedURL.Host == "" {
		return nil, nil, errors.New("mattermost websocket host is required")
	}

	address := parsedURL.Host
	if !strings.Contains(address, ":") {
		if parsedURL.Scheme == "wss" {
			address += ":443"
		} else {
			address += ":80"
		}
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	connection, errorValue := dialer.DialContext(ctx, "tcp", address)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if errorValue := connection.SetDeadline(time.Now().Add(defaultMattermostWebSocketHandshakeTimeout)); errorValue != nil {
		_ = connection.Close()
		return nil, nil, errorValue
	}

	if parsedURL.Scheme == "wss" {
		tlsConnection := tls.Client(connection, &tls.Config{ServerName: parsedURL.Hostname(), MinVersion: tls.VersionTLS12})
		if errorValue := tlsConnection.HandshakeContext(ctx); errorValue != nil {
			_ = connection.Close()
			return nil, nil, errorValue
		}
		connection = tlsConnection
	}

	reader := bufio.NewReader(connection)
	secWebSocketKey, errorValue := randomWebSocketKey()
	if errorValue != nil {
		_ = connection.Close()
		return nil, nil, errorValue
	}

	requestPath := parsedURL.RequestURI()
	if requestPath == "" {
		requestPath = "/api/v4/websocket"
	}
	request := "GET " + requestPath + " HTTP/1.1\r\n" +
		"Host: " + parsedURL.Host + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Key: " + secWebSocketKey + "\r\n" +
		"Sec-WebSocket-Version: 13\r\n\r\n"
	if strings.TrimSpace(forwarder.BotToken) != "" {
		request = strings.TrimSuffix(request, "\r\n\r\n") + "\r\nAuthorization: Bearer " + forwarder.BotToken + "\r\n\r\n"
	}
	if _, errorValue := connection.Write([]byte(request)); errorValue != nil {
		_ = connection.Close()
		return nil, nil, errorValue
	}

	httpResponse, errorValue := http.ReadResponse(reader, nil)
	if errorValue != nil {
		_ = connection.Close()
		return nil, nil, errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusSwitchingProtocols {
		_ = connection.Close()
		return nil, nil, errors.New("mattermost websocket upgrade failed: " + httpResponse.Status)
	}
	if httpResponse.Header.Get("Sec-WebSocket-Accept") != expectedWebSocketAccept(secWebSocketKey) {
		_ = connection.Close()
		return nil, nil, errors.New("mattermost websocket accept key mismatch")
	}
	if errorValue := connection.SetDeadline(time.Time{}); errorValue != nil {
		_ = connection.Close()
		return nil, nil, errorValue
	}

	return connection, reader, nil
}

func (forwarder MattermostWebSocketForwarder) authenticationChallenge() []byte {
	document, _ := json.Marshal(map[string]any{
		"seq":    1,
		"action": "authentication_challenge",
		"data": map[string]string{
			"token": forwarder.BotToken,
		},
	})
	return document
}

func (forwarder MattermostWebSocketForwarder) forward(ctx context.Context, payload []byte) error {
	eventBuilder := forwarder.EventBuilder
	if eventBuilder == nil {
		eventBuilder = func(_ context.Context, payload []byte) (platformInboundEvent, bool, error) {
			return normalizeMattermostWebSocketPayload(payload, forwarder.BotUserID, forwarder.BotUsername)
		}
	}
	event, hasEvent, errorValue := eventBuilder(ctx, payload)
	if errorValue != nil || !hasEvent {
		return errorValue
	}
	document, errorValue := json.Marshal(event)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, forwarder.BlueclawURL, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	client := forwarder.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseDocument, _ := io.ReadAll(response.Body)
		return errors.New(string(responseDocument))
	}
	if forwarder.AfterForward != nil {
		forwarder.AfterForward(ctx, payload)
	}
	return nil
}

func randomWebSocketKey() (string, error) {
	value := make([]byte, 16)
	_, errorValue := rand.Read(value)
	if errorValue != nil {
		return "", errorValue
	}
	return base64.StdEncoding.EncodeToString(value), nil
}

func expectedWebSocketAccept(key string) string {
	sum := sha1.Sum([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func readWebSocketFrame(writer io.Writer, reader *bufio.Reader) ([]byte, error) {
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
		return nil, errors.New("websocket close: " + string(payload))
	case 0x9:
		_ = writeWebSocketFrame(writer, 0xA, payload)
		return nil, nil
	case 0xA:
		return nil, nil
	default:
		return nil, nil
	}
}

func writeWebSocketTextFrame(writer io.Writer, payload []byte) error {
	return writeWebSocketFrame(writer, 0x1, payload)
}

func writeWebSocketFrame(writer io.Writer, opcode byte, payload []byte) error {
	maskKey := make([]byte, 4)
	if _, errorValue := rand.Read(maskKey); errorValue != nil {
		return errorValue
	}

	header := []byte{0x80 | opcode}
	payloadLength := len(payload)
	switch {
	case payloadLength < 126:
		header = append(header, 0x80|byte(payloadLength))
	case payloadLength <= 65535:
		header = append(header, 0x80|126)
		lengthBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(lengthBytes, uint16(payloadLength))
		header = append(header, lengthBytes...)
	default:
		header = append(header, 0x80|127)
		lengthBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(lengthBytes, uint64(payloadLength))
		header = append(header, lengthBytes...)
	}
	header = append(header, maskKey...)

	maskedPayload := make([]byte, payloadLength)
	for index, value := range payload {
		maskedPayload[index] = value ^ maskKey[index%4]
	}

	_, errorValue := writer.Write(append(header, maskedPayload...))
	return errorValue
}
