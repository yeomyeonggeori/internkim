package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type signalJSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int64  `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type signalJSONRPCResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type signalReceiveEnvelope struct {
	Envelope struct {
		Source      string `json:"source"`
		SourceName  string `json:"sourceName"`
		Timestamp   int64  `json:"timestamp"`
		DataMessage struct {
			Message   string `json:"message"`
			Timestamp int64  `json:"timestamp"`
			GroupInfo struct {
				GroupID string `json:"groupId"`
			} `json:"groupInfo"`
		} `json:"dataMessage"`
		SyncMessage struct {
			SentMessage struct {
				Message string `json:"message"`
			} `json:"sentMessage"`
		} `json:"syncMessage"`
	} `json:"envelope"`
}

var signalJSONRPCID int64

func (service Service) startSignalJSONRPCReceiver(ctx context.Context) {
	if strings.TrimSpace(service.Configuration.SignalJSONRPCURL) == "" || strings.TrimSpace(service.Configuration.SignalAccount) == "" {
		log.Print("signal jsonrpc receiver disabled: configuration missing")
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if errorValue := service.pollSignal(ctx); errorValue != nil {
			log.Printf("signal jsonrpc poll failed: %v", errorValue)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (service Service) pollSignal(ctx context.Context) error {
	result, errorValue := service.signalJSONRPC(ctx, "receive", map[string]any{
		"account": service.Configuration.SignalAccount,
		"timeout": 1,
	})
	if errorValue != nil {
		return errorValue
	}
	var envelopes []signalReceiveEnvelope
	if errorValue := json.Unmarshal(result, &envelopes); errorValue != nil {
		return errorValue
	}
	for _, envelope := range envelopes {
		event, hasEvent, errorValue := service.normalizeSignalEnvelope(envelope)
		if errorValue != nil {
			log.Printf("signal normalize failed: %v", errorValue)
			continue
		}
		if !hasEvent {
			continue
		}
		if errorValue := service.forwardPlatformEvent(ctx, "signal", event); errorValue != nil {
			log.Printf("signal event forward failed: %v", errorValue)
		}
	}
	return nil
}

func (service Service) normalizeSignalEnvelope(envelope signalReceiveEnvelope) (platformInboundEvent, bool, error) {
	source := strings.TrimSpace(envelope.Envelope.Source)
	message := strings.TrimSpace(envelope.Envelope.DataMessage.Message)
	if source == "" || message == "" {
		return platformInboundEvent{}, false, nil
	}
	timestamp := envelope.Envelope.DataMessage.Timestamp
	if timestamp == 0 {
		timestamp = envelope.Envelope.Timestamp
	}
	if timestamp == 0 {
		return platformInboundEvent{}, false, nil
	}

	groupID := strings.TrimSpace(envelope.Envelope.DataMessage.GroupInfo.GroupID)
	conversationID := "dm:" + source
	if groupID != "" {
		conversationID = "group:" + groupID
	}
	handle := platformHandle{
		Platform:        "signal",
		ConversationID:  conversationID,
		MessageID:       strconv.FormatInt(timestamp, 10),
		SignalAccount:   service.Configuration.SignalAccount,
		SignalRecipient: source,
		SignalGroupID:   groupID,
	}
	replyTargetID, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	historyCursor, errorValue := encodePlatformHandle(handle)
	if errorValue != nil {
		return platformInboundEvent{}, false, errorValue
	}
	return platformInboundEvent{
		ConversationID: conversationID,
		MessageID:      strconv.FormatInt(timestamp, 10),
		SenderID:       source,
		ReplyTargetID:  replyTargetID,
		Prompt:         message,
		Context: platformEventContext{
			Messages:      nil,
			HasMoreBefore: false,
			HistoryCursor: historyCursor,
		},
	}, true, nil
}

func (service Service) signalLookupUserFromRequest(_ context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	var request userLookupRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	senderID := strings.TrimSpace(request.SenderID)
	if senderID == "" {
		return nil, errors.New("senderID is required")
	}
	return map[string]string{
		"platform":    "signal",
		"senderID":    senderID,
		"userID":      senderID,
		"displayName": senderID,
	}, nil
}

func (service Service) signalReplyFromRequest(ctx context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	var request replyRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	handle, errorValue := decodePlatformHandle(request.ReplyTargetID)
	if errorValue != nil {
		return nil, errorValue
	}
	if handle.Platform != "signal" {
		return nil, errors.New("reply target platform mismatch")
	}
	params := map[string]any{
		"account": service.Configuration.SignalAccount,
		"message": request.Message,
	}
	if strings.TrimSpace(handle.SignalGroupID) != "" {
		params["groupId"] = handle.SignalGroupID
	} else {
		params["recipients"] = []string{handle.SignalRecipient}
	}
	result, errorValue := service.signalJSONRPC(ctx, "send", params)
	if errorValue != nil {
		return nil, errorValue
	}
	return map[string]string{"dispatchID": strings.TrimSpace(string(result))}, nil
}

func (service Service) signalHistoryFromRequest(_ context.Context, reader io.Reader) (any, error) {
	payload, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return nil, errorValue
	}
	var request historyFetchRequest
	if errorValue := json.Unmarshal(payload, &request); errorValue != nil {
		return nil, errorValue
	}
	handle, errorValue := decodePlatformHandle(request.HistoryCursor)
	if errorValue != nil {
		return nil, errorValue
	}
	if handle.Platform != "signal" {
		return nil, errors.New("history cursor platform mismatch")
	}
	return map[string]any{
		"messages":      []platformContextMessage{},
		"hasMoreBefore": false,
		"historyCursor": "",
	}, nil
}

func (service Service) signalJSONRPC(ctx context.Context, method string, params any) (json.RawMessage, error) {
	requestDocument, errorValue := json.Marshal(signalJSONRPCRequest{
		JSONRPC: "2.0",
		ID:      atomic.AddInt64(&signalJSONRPCID, 1),
		Method:  method,
		Params:  params,
	})
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, service.Configuration.SignalJSONRPCURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, _ := io.ReadAll(httpResponse.Body)
	if httpResponse.StatusCode < 200 || httpResponse.StatusCode >= 300 {
		return nil, errors.New(string(responseDocument))
	}
	var response signalJSONRPCResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return nil, errorValue
	}
	if response.Error != nil {
		return nil, errors.New(response.Error.Message)
	}
	return response.Result, nil
}
