package capabilityd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type slackSocketEnvelope struct {
	EnvelopeID string `json:"envelope_id"`
	Type       string `json:"type"`
	Payload    struct {
		EventID string `json:"event_id"`
		Event   struct {
			Type        string `json:"type"`
			Subtype     string `json:"subtype"`
			User        string `json:"user"`
			BotID       string `json:"bot_id"`
			Channel     string `json:"channel"`
			ChannelType string `json:"channel_type"`
			Text        string `json:"text"`
			TS          string `json:"ts"`
			ThreadTS    string `json:"thread_ts"`
		} `json:"event"`
	} `json:"payload"`
}

func (service Service) startSlackSocketMode(ctx context.Context) {
	appToken := readSecretValue(service.Configuration.SlackAppTokenPath)
	botToken := readSecretValue(service.Configuration.SlackTokenPath)
	if appToken == "" || botToken == "" {
		log.Print("slack socket mode disabled: token missing")
		return
	}

	botUserID, errorValue := service.slackBotUserID(ctx)
	if errorValue != nil {
		log.Printf("slack socket mode disabled: bot lookup failed: %v", errorValue)
		return
	}

	backoff := time.Second
	for ctx.Err() == nil {
		errorValue := service.runSlackSocketMode(ctx, appToken, botUserID)
		if errorValue != nil && ctx.Err() == nil {
			log.Printf("slack socket mode disconnected: %v", errorValue)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (service Service) runSlackSocketMode(ctx context.Context, appToken string, botUserID string) error {
	socketURL, errorValue := service.slackSocketModeURL(ctx, appToken)
	if errorValue != nil {
		return errorValue
	}

	connection, reader, errorValue := dialWebSocket(ctx, socketURL, nil)
	if errorValue != nil {
		return errorValue
	}
	defer connection.Close()

	for ctx.Err() == nil {
		payload, errorValue := readWebSocketFrame(connection, reader)
		if errorValue != nil {
			return errorValue
		}
		if errorValue := service.handleSlackSocketEnvelope(ctx, connection, payload, botUserID); errorValue != nil {
			log.Printf("slack socket envelope failed: %v", errorValue)
		}
	}
	return ctx.Err()
}

func (service Service) slackSocketModeURL(ctx context.Context, appToken string) (string, error) {
	var response struct {
		IsOK  bool   `json:"ok"`
		URL   string `json:"url"`
		Error string `json:"error"`
	}
	errorValue := service.authenticatedJSONRequest(ctx, http.MethodPost, "https://slack.com/api/apps.connections.open", appToken, nil, &response)
	if errorValue != nil {
		return "", errorValue
	}
	if !response.IsOK {
		return "", errors.New("slack socket open failed: " + response.Error)
	}
	return response.URL, nil
}

func (service Service) handleSlackSocketEnvelope(ctx context.Context, writer io.Writer, payload []byte, botUserID string) error {
	var envelope slackSocketEnvelope
	if errorValue := json.Unmarshal(payload, &envelope); errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(envelope.EnvelopeID) != "" {
		ackDocument, _ := json.Marshal(map[string]string{"envelope_id": envelope.EnvelopeID})
		if errorValue := writeWebSocketTextFrame(writer, ackDocument); errorValue != nil {
			return errorValue
		}
	}

	event, hasEvent, errorValue := service.normalizeSlackEnvelope(ctx, envelope, botUserID)
	if errorValue != nil || !hasEvent {
		return errorValue
	}
	return service.forwardPlatformEvent(ctx, "slack", event)
}

func (service Service) normalizeSlackEnvelope(ctx context.Context, envelope slackSocketEnvelope, botUserID string) (platformInboundEvent, bool, error) {
	event := envelope.Payload.Event
	if event.Type != "message" {
		return platformInboundEvent{}, false, nil
	}
	if strings.TrimSpace(event.Subtype) != "" || strings.TrimSpace(event.BotID) != "" || strings.TrimSpace(event.User) == strings.TrimSpace(botUserID) {
		return platformInboundEvent{}, false, nil
	}
	if strings.TrimSpace(event.Channel) == "" || strings.TrimSpace(event.TS) == "" || strings.TrimSpace(event.User) == "" {
		return platformInboundEvent{}, false, nil
	}

	handle := slackPlatformHandle(event.Channel, event.ChannelType, event.TS, event.ThreadTS)
	replyTargetID, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	contextValue := service.slackContext(ctx, handle, 20)
	return platformInboundEvent{
		ConversationID: handle.ConversationID,
		MessageID:      event.TS,
		SenderID:       event.User,
		ReplyTargetID:  replyTargetID,
		Prompt:         event.Text,
		Context:        contextValue,
	}, true, nil
}

func slackPlatformHandle(channelID string, channelType string, messageTS string, threadTS string) platformHandle {
	threadTimestamp := strings.TrimSpace(threadTS)
	if threadTimestamp == "" && !isSlackDirectConversation(channelType) {
		threadTimestamp = messageTS
	}
	conversationID := namespacedConversationID(channelType, channelID)
	if threadTimestamp != "" && threadTimestamp != messageTS {
		conversationID = "thread:" + channelID + ":" + threadTimestamp
	}
	if threadTimestamp == messageTS && !isSlackDirectConversation(channelType) {
		conversationID = "thread:" + channelID + ":" + threadTimestamp
	}
	return platformHandle{
		Platform:        "slack",
		ConversationID:  conversationID,
		ChannelID:       channelID,
		ChannelType:     channelType,
		ThreadTimestamp: threadTimestamp,
		MessageTS:       messageTS,
	}
}

func isSlackDirectConversation(channelType string) bool {
	return strings.EqualFold(strings.TrimSpace(channelType), "im")
}

func (service Service) slackContext(ctx context.Context, handle platformHandle, limit int) platformEventContext {
	limit = normalizedHistoryLimit(limit)
	messages, hasMoreBefore, errorValue := service.slackHistoryMessages(ctx, handle, limit+1)
	if errorValue != nil {
		return platformEventContext{HistoryCursor: mustEncodePlatformHandle(handle)}
	}
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
		hasMoreBefore = true
	}
	contextValue := platformEventContext{Messages: messages, HasMoreBefore: hasMoreBefore}
	if hasMoreBefore {
		contextValue.HistoryCursor = mustEncodePlatformHandle(handle)
	}
	return contextValue
}

func (service Service) slackHistoryMessages(ctx context.Context, handle platformHandle, limit int) ([]platformContextMessage, bool, error) {
	var response struct {
		IsOK     bool `json:"ok"`
		Messages []struct {
			User string `json:"user"`
			Text string `json:"text"`
			TS   string `json:"ts"`
		} `json:"messages"`
		HasMore bool   `json:"has_more"`
		Error   string `json:"error"`
	}
	limit = normalizedHistoryLimit(limit)
	path := "/conversations.history?channel=" + url.QueryEscape(handle.ChannelID) + "&limit=" + strconv.Itoa(limit)
	if strings.TrimSpace(handle.ThreadTimestamp) != "" {
		path = "/conversations.replies?channel=" + url.QueryEscape(handle.ChannelID) + "&ts=" + url.QueryEscape(handle.ThreadTimestamp) + "&limit=" + strconv.Itoa(limit)
	}
	if errorValue := service.slackRequest(ctx, http.MethodGet, path, nil, &response); errorValue != nil {
		return nil, false, errorValue
	}
	if !response.IsOK {
		return nil, false, errors.New("slack history fetch failed: " + response.Error)
	}

	filteredMessages := response.Messages[:0]
	for _, message := range response.Messages {
		if message.TS == handle.MessageTS {
			continue
		}
		filteredMessages = append(filteredMessages, message)
	}
	sort.SliceStable(filteredMessages, func(leftIndex int, rightIndex int) bool {
		return filteredMessages[leftIndex].TS < filteredMessages[rightIndex].TS
	})

	speakerByUserID := map[string]string{}
	messages := make([]platformContextMessage, 0, len(filteredMessages))
	for _, message := range filteredMessages {
		if strings.TrimSpace(message.Text) == "" {
			continue
		}
		speaker := speakerByUserID[message.User]
		if speaker == "" {
			speaker = service.slackSpeaker(ctx, message.User)
			speakerByUserID[message.User] = speaker
		}
		messages = append(messages, platformContextMessage{Speaker: speaker, Text: message.Text})
	}
	return messages, response.HasMore, nil
}

func (service Service) slackSpeaker(ctx context.Context, userID string) string {
	if strings.TrimSpace(userID) == "" {
		return "unknown"
	}
	var response struct {
		IsOK bool `json:"ok"`
		User struct {
			Name     string `json:"name"`
			RealName string `json:"real_name"`
		} `json:"user"`
	}
	errorValue := service.slackRequest(ctx, http.MethodGet, "/users.info?user="+url.QueryEscape(userID), nil, &response)
	if errorValue != nil || !response.IsOK {
		return userID
	}
	return firstNonEmpty(response.User.RealName, response.User.Name, userID)
}

func (service Service) slackBotUserID(ctx context.Context) (string, error) {
	var response struct {
		IsOK   bool   `json:"ok"`
		UserID string `json:"user_id"`
		Error  string `json:"error"`
	}
	errorValue := service.slackRequest(ctx, http.MethodPost, "/auth.test", nil, &response)
	if errorValue != nil {
		return "", errorValue
	}
	if !response.IsOK {
		return "", errors.New("slack auth failed: " + response.Error)
	}
	return response.UserID, nil
}

func dialWebSocket(ctx context.Context, rawURL string, headers map[string]string) (net.Conn, *bufio.Reader, error) {
	parsedURL, errorValue := url.Parse(rawURL)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	if parsedURL.Host == "" {
		return nil, nil, errors.New("websocket host is required")
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
		requestPath = "/"
	}
	requestBuffer := bytes.NewBufferString("GET " + requestPath + " HTTP/1.1\r\n")
	requestBuffer.WriteString("Host: " + parsedURL.Host + "\r\n")
	requestBuffer.WriteString("Upgrade: websocket\r\n")
	requestBuffer.WriteString("Connection: Upgrade\r\n")
	requestBuffer.WriteString("Sec-WebSocket-Key: " + secWebSocketKey + "\r\n")
	requestBuffer.WriteString("Sec-WebSocket-Version: 13\r\n")
	for headerName, headerValue := range headers {
		requestBuffer.WriteString(headerName + ": " + headerValue + "\r\n")
	}
	requestBuffer.WriteString("\r\n")

	if _, errorValue := connection.Write(requestBuffer.Bytes()); errorValue != nil {
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
		return nil, nil, errors.New("websocket upgrade failed: " + httpResponse.Status)
	}
	if httpResponse.Header.Get("Sec-WebSocket-Accept") != expectedWebSocketAccept(secWebSocketKey) {
		_ = connection.Close()
		return nil, nil, errors.New("websocket accept key mismatch")
	}
	return connection, reader, nil
}
