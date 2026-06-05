package admind

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	messagemail "github.com/emersion/go-message/mail"
)

type mailMessageListRequest struct {
	Mailbox   string
	Query     string
	Limit     int
	BeforeUID uint32
}

type mailMessageCursor struct {
	Mailbox   string `json:"mailbox"`
	Query     string `json:"query"`
	BeforeUID uint32 `json:"beforeUID"`
}

type mailMessageSendRequest struct {
	To      []string `json:"to"`
	CC      []string `json:"cc"`
	BCC     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

type mailMessageMoveRequest struct {
	TargetMailbox string `json:"targetMailbox"`
}

type mailMessageMarkRequest struct {
	Seen    *bool `json:"seen"`
	Flagged *bool `json:"flagged"`
}

func mailMessageListRequestFromURL(request *http.Request) (mailMessageListRequest, error) {
	limit := 50
	if value := strings.TrimSpace(request.URL.Query().Get("limit")); value != "" {
		parsedLimit, errorValue := strconv.Atoi(value)
		if errorValue != nil {
			return mailMessageListRequest{}, errors.New("limit must be a number")
		}
		limit = parsedLimit
	}
	if limit < 1 || limit > 100 {
		return mailMessageListRequest{}, errors.New("limit must be between 1 and 100")
	}
	input := mailMessageListRequest{
		Mailbox: strings.TrimSpace(firstNonEmpty(request.URL.Query().Get("mailbox"), "INBOX")),
		Query:   strings.TrimSpace(request.URL.Query().Get("query")),
		Limit:   limit,
	}
	cursor := strings.TrimSpace(request.URL.Query().Get("cursor"))
	if cursor == "" {
		return input, nil
	}
	decodedCursor, errorValue := decodeMailMessageCursor(cursor)
	if errorValue != nil ||
		decodedCursor.BeforeUID == 0 ||
		decodedCursor.Mailbox != input.Mailbox ||
		decodedCursor.Query != input.Query {
		return mailMessageListRequest{}, errors.New("invalid cursor")
	}
	input.BeforeUID = decodedCursor.BeforeUID
	return input, nil
}

func encodeMailMessageCursor(mailbox string, query string, beforeUID uint32) string {
	if beforeUID == 0 {
		return ""
	}
	document, errorValue := json.Marshal(mailMessageCursor{
		Mailbox:   strings.TrimSpace(mailbox),
		Query:     strings.TrimSpace(query),
		BeforeUID: beforeUID,
	})
	if errorValue != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(document)
}

func decodeMailMessageCursor(value string) (mailMessageCursor, error) {
	document, errorValue := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if errorValue != nil {
		return mailMessageCursor{}, errorValue
	}
	var cursor mailMessageCursor
	if errorValue := json.Unmarshal(document, &cursor); errorValue != nil {
		return mailMessageCursor{}, errorValue
	}
	cursor.Mailbox = strings.TrimSpace(cursor.Mailbox)
	cursor.Query = strings.TrimSpace(cursor.Query)
	return cursor, nil
}

func mailMessagePathParts(request *http.Request, suffix string) (string, uint32, error) {
	trimmedPath := strings.TrimSuffix(strings.TrimPrefix(request.URL.EscapedPath(), "/mail/api/messages/"), suffix)
	parts := strings.Split(strings.Trim(trimmedPath, "/"), "/")
	if len(parts) != 2 {
		return "", 0, errors.New("mailbox and uid are required")
	}
	mailbox, errorValue := url.PathUnescape(strings.TrimSpace(parts[0]))
	if errorValue != nil {
		return "", 0, errors.New("mailbox is invalid")
	}
	uid, errorValue := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 32)
	if mailbox == "" || errorValue != nil || uid == 0 {
		return "", 0, errors.New("mailbox and uid are required")
	}
	return mailbox, uint32(uid), nil
}

func validateMailMessageSendRequest(input mailMessageSendRequest) (mailMessageSendRequest, error) {
	var errorValue error
	input.To, errorValue = validateMailAddressList(input.To, "to")
	if errorValue != nil {
		return mailMessageSendRequest{}, errorValue
	}
	input.CC, errorValue = validateMailAddressList(input.CC, "cc")
	if errorValue != nil {
		return mailMessageSendRequest{}, errorValue
	}
	input.BCC, errorValue = validateMailAddressList(input.BCC, "bcc")
	if errorValue != nil {
		return mailMessageSendRequest{}, errorValue
	}
	input.Subject = strings.TrimSpace(input.Subject)
	input.Body = strings.TrimSpace(input.Body)
	if len(input.To) == 0 {
		return mailMessageSendRequest{}, errors.New("to is required")
	}
	if input.Subject == "" && input.Body == "" {
		return mailMessageSendRequest{}, errors.New("subject or body is required")
	}
	return input, nil
}

func validateMailAddressList(values []string, fieldName string) ([]string, error) {
	addresses := []string{}
	seenAddresses := map[string]bool{}
	for _, value := range values {
		address := strings.TrimSpace(value)
		if address == "" {
			continue
		}
		parsedAddress, errorValue := messagemail.ParseAddress(address)
		if errorValue != nil {
			return nil, fmt.Errorf("%s contains an invalid address: %w", fieldName, errorValue)
		}
		normalizedAddress := strings.ToLower(parsedAddress.Address)
		if seenAddresses[normalizedAddress] {
			continue
		}
		seenAddresses[normalizedAddress] = true
		addresses = append(addresses, parsedAddress.String())
	}
	return addresses, nil
}
