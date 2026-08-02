package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type mailMessageListInput struct {
	Mailbox string `json:"mailbox"`
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Cursor  string `json:"cursor"`
}

type mailMessageReadInput struct {
	Mailbox string `json:"mailbox"`
	UID     string `json:"uid"`
}

type mailMessageSendInput struct {
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	CC      []string `json:"cc"`
	BCC     []string `json:"bcc"`
}

type mailMessageMoveInput struct {
	Mailbox       string `json:"mailbox"`
	UID           string `json:"uid"`
	TargetMailbox string `json:"targetMailbox"`
}

type mailMessageMarkInput struct {
	Mailbox string `json:"mailbox"`
	UID     string `json:"uid"`
	Seen    *bool  `json:"seen"`
	Flagged *bool  `json:"flagged"`
}

func (service Service) invokeMailTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	result, errorValue := service.invokeMail(ctx, request)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          "ok",
		Result:          result,
	}, nil
}

func (service Service) invokeMail(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "mail_message_list":
		return service.invokeMailMessageList(ctx, request)
	case "mail_message_search":
		return service.invokeMailMessageSearch(ctx, request)
	case "mail_message_read":
		return service.invokeMailMessageRead(ctx, request)
	case "mail_message_send":
		return service.invokeMailMessageSend(ctx, request)
	case "mail_message_move":
		return service.invokeMailMessageMove(ctx, request)
	case "mail_message_mark":
		return service.invokeMailMessageMark(ctx, request)
	case "mail_connection_status":
		return service.invokeMailConnectionStatus(ctx, request)
	case "mail_connection_start":
		return service.invokeMailConnectionStart(ctx, request)
	default:
		return nil, fmt.Errorf("mail tool is not configured: %s", request.ToolName)
	}
}

func (service Service) invokeMailMessageList(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	input, errorValue := decodeMailMessageListInput(request.Input)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.sendMailToolRequest(ctx, http.MethodGet, mailMessageListPath(input), nil, request.Context.RequesterEmail)
}

func (service Service) invokeMailMessageSearch(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	input, errorValue := decodeMailMessageSearchInput(request.Input)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.sendMailToolRequest(ctx, http.MethodGet, mailMessageListPath(input), nil, request.Context.RequesterEmail)
}

func (service Service) invokeMailMessageRead(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	input, errorValue := decodeMailMessageReadInput(request.Input)
	if errorValue != nil {
		return nil, errorValue
	}
	path := "/mail/api/messages/" + url.PathEscape(input.Mailbox) + "/" + url.PathEscape(input.UID)
	return service.sendMailToolRequest(ctx, http.MethodGet, path, nil, request.Context.RequesterEmail)
}

func (service Service) invokeMailMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	input, errorValue := decodeMailMessageSendInput(request.Input)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.sendMailToolRequest(ctx, http.MethodPost, "/mail/api/messages/send", input, request.Context.RequesterEmail)
}

func (service Service) invokeMailMessageMove(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	input, errorValue := decodeMailMessageMoveInput(request.Input)
	if errorValue != nil {
		return nil, errorValue
	}
	path := "/mail/api/messages/" + url.PathEscape(input.Mailbox) + "/" + url.PathEscape(input.UID) + "/move"
	return service.sendMailToolRequest(ctx, http.MethodPost, path, input, request.Context.RequesterEmail)
}

func (service Service) invokeMailMessageMark(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	input, errorValue := decodeMailMessageMarkInput(request.Input)
	if errorValue != nil {
		return nil, errorValue
	}
	path := "/mail/api/messages/" + url.PathEscape(input.Mailbox) + "/" + url.PathEscape(input.UID) + "/flags"
	return service.sendMailToolRequest(ctx, http.MethodPost, path, input, request.Context.RequesterEmail)
}

func (service Service) invokeMailConnectionStatus(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	return service.sendMailToolRequest(ctx, http.MethodGet, "/mail/api/account", nil, request.Context.RequesterEmail)
}

func (service Service) invokeMailConnectionStart(ctx context.Context, request capabilities.ToolInvokeRequest) (json.RawMessage, error) {
	baseURL := strings.TrimRight(service.Configuration.AdmindBaseURL, "/")
	document, errorValue := json.Marshal(map[string]any{
		"status":   "configuration_required",
		"provider": "manual",
		"setupURL": baseURL + "/mail/",
	})
	if errorValue != nil {
		return nil, errorValue
	}
	return document, nil
}

func decodeMailMessageListInput(document json.RawMessage) (mailMessageListInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return mailMessageListInput{Mailbox: "INBOX", Limit: 20}, nil
	}
	var input mailMessageListInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mailMessageListInput{}, errorValue
	}
	input.Mailbox = firstNonEmpty(strings.TrimSpace(input.Mailbox), "INBOX")
	input.Query = strings.TrimSpace(input.Query)
	input.Cursor = strings.TrimSpace(input.Cursor)
	if input.Limit == 0 {
		input.Limit = 20
	}
	if input.Limit < 1 || input.Limit > 50 {
		return mailMessageListInput{}, fmt.Errorf("limit must be between 1 and 50")
	}
	return input, nil
}

func decodeMailMessageSearchInput(document json.RawMessage) (mailMessageListInput, error) {
	input, errorValue := decodeMailMessageListInput(document)
	if errorValue != nil {
		return mailMessageListInput{}, errorValue
	}
	if strings.TrimSpace(input.Query) == "" {
		return mailMessageListInput{}, fmt.Errorf("query is required")
	}
	return input, nil
}

func decodeMailMessageReadInput(document json.RawMessage) (mailMessageReadInput, error) {
	var input mailMessageReadInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mailMessageReadInput{}, errorValue
	}
	input.Mailbox = strings.TrimSpace(input.Mailbox)
	input.UID = strings.TrimSpace(input.UID)
	if input.Mailbox == "" || input.UID == "" {
		return mailMessageReadInput{}, fmt.Errorf("mailbox and uid are required")
	}
	return input, nil
}

func decodeMailMessageSendInput(document json.RawMessage) (mailMessageSendInput, error) {
	var input mailMessageSendInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mailMessageSendInput{}, errorValue
	}
	input.Subject = strings.TrimSpace(input.Subject)
	input.Body = strings.TrimSpace(input.Body)
	input.To = normalizeMailAddresses(input.To)
	input.CC = normalizeMailAddresses(input.CC)
	input.BCC = normalizeMailAddresses(input.BCC)
	if len(input.To) == 0 {
		return mailMessageSendInput{}, fmt.Errorf("to is required")
	}
	if input.Subject == "" && input.Body == "" {
		return mailMessageSendInput{}, fmt.Errorf("subject or body is required")
	}
	return input, nil
}

func decodeMailMessageMoveInput(document json.RawMessage) (mailMessageMoveInput, error) {
	var input mailMessageMoveInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mailMessageMoveInput{}, errorValue
	}
	input.Mailbox = strings.TrimSpace(input.Mailbox)
	input.UID = strings.TrimSpace(input.UID)
	input.TargetMailbox = strings.TrimSpace(input.TargetMailbox)
	if input.Mailbox == "" || input.UID == "" || input.TargetMailbox == "" {
		return mailMessageMoveInput{}, fmt.Errorf("mailbox, uid, and targetMailbox are required")
	}
	return input, nil
}

func decodeMailMessageMarkInput(document json.RawMessage) (mailMessageMarkInput, error) {
	var input mailMessageMarkInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return mailMessageMarkInput{}, errorValue
	}
	input.Mailbox = strings.TrimSpace(input.Mailbox)
	input.UID = strings.TrimSpace(input.UID)
	if input.Mailbox == "" || input.UID == "" {
		return mailMessageMarkInput{}, fmt.Errorf("mailbox and uid are required")
	}
	return input, nil
}

func normalizeMailAddresses(values []string) []string {
	addresses := []string{}
	seenAddresses := map[string]bool{}
	for _, value := range values {
		address := strings.ToLower(strings.TrimSpace(value))
		if address == "" || seenAddresses[address] {
			continue
		}
		seenAddresses[address] = true
		addresses = append(addresses, address)
	}
	return addresses
}

func mailMessageListPath(input mailMessageListInput) string {
	query := url.Values{}
	query.Set("mailbox", input.Mailbox)
	query.Set("limit", fmt.Sprintf("%d", input.Limit))
	if input.Query != "" {
		query.Set("query", input.Query)
	}
	if input.Cursor != "" {
		query.Set("cursor", input.Cursor)
	}
	return "/mail/api/messages?" + query.Encode()
}

func (service Service) sendMailToolRequest(ctx context.Context, method string, path string, payload any, requesterEmail string) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		document, errorValue := json.Marshal(payload)
		if errorValue != nil {
			return nil, errorValue
		}
		body = bytes.NewReader(document)
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(ctx, method, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+path, body)
	if errorValue != nil {
		return nil, errorValue
	}
	if payload != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	setMailRequesterEmailHeader(httpRequest, requesterEmail)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	responseBody, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("mail tool failed: %s", strings.TrimSpace(string(responseBody)))
	}
	return json.RawMessage(responseBody), nil
}

func setMailRequesterEmailHeader(request *http.Request, requesterEmail string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedEmail == "" {
		return
	}
	request.Header.Set("CF-Access-Authenticated-User-Email", normalizedEmail)
}
