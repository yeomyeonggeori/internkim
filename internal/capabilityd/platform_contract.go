package capabilityd

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

type platformInboundEvent struct {
	ConversationID string               `json:"conversationID"`
	MessageID      string               `json:"messageID"`
	SenderID       string               `json:"senderID"`
	ReplyTargetID  string               `json:"replyTargetID"`
	Prompt         string               `json:"prompt"`
	InputParts     []platformPart       `json:"inputParts,omitempty"`
	Context        platformEventContext `json:"context"`
}

type platformEventContext struct {
	Messages         []platformContextMessage  `json:"messages"`
	HasMoreBefore    bool                      `json:"hasMoreBefore"`
	HistoryCursor    string                    `json:"historyCursor,omitempty"`
	Sender           platformContextSender     `json:"sender,omitempty"`
	ReceivedAt       string                    `json:"receivedAt,omitempty"`
	ConversationType string                    `json:"conversationType,omitempty"`
	ChannelID        string                    `json:"channelID,omitempty"`
	ChannelName      string                    `json:"channelName,omitempty"`
	Addressing       platformAddressing        `json:"addressing,omitempty"`
	AttachmentsOnly  bool                      `json:"attachmentsOnly,omitempty"`
	InputAttachments []platformInputAttachment `json:"inputAttachments,omitempty"`
	Materials        []platformInputAttachment `json:"materials,omitempty"`
}

type platformAddressing struct {
	BotMentioned         bool `json:"botMentioned,omitempty"`
	OtherPersonMentioned bool `json:"otherPersonMentioned,omitempty"`
}

type platformInputAttachment struct {
	Platform    string `json:"platform,omitempty"`
	FileID      string `json:"fileID,omitempty"`
	MessageID   string `json:"messageID,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"contentType,omitempty"`
	SizeBytes   int64  `json:"sizeBytes,omitempty"`
	Path        string `json:"path,omitempty"`
	// ContentBase64 carries the fetched file to the workspace it belongs in.
	// This daemon has no identity there to write it as.
	ContentBase64 string `json:"contentBase64,omitempty"`
	IsAvailable   bool   `json:"isAvailable,omitempty"`
	ErrorCode     string `json:"errorCode,omitempty"`
	Message       string `json:"message,omitempty"`
}

type platformPart struct {
	Type       string             `json:"type"`
	Text       string             `json:"text,omitempty"`
	Image      *platformImagePart `json:"image,omitempty"`
	File       *platformFilePart  `json:"file,omitempty"`
	Source     platformPartSource `json:"source,omitempty"`
	Visibility string             `json:"visibility,omitempty"`
}

type platformImagePart struct {
	MimeType   string `json:"mimeType,omitempty"`
	DataBase64 string `json:"dataBase64,omitempty"`
	Path       string `json:"path,omitempty"`
	Filename   string `json:"filename,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
}

type platformFilePart struct {
	Path              string `json:"path,omitempty"`
	Filename          string `json:"filename,omitempty"`
	ContentType       string `json:"contentType,omitempty"`
	SizeBytes         int64  `json:"sizeBytes,omitempty"`
	MarkdownPreview   string `json:"markdownPreview,omitempty"`
	ConversionStatus  string `json:"conversionStatus,omitempty"`
	ConversionMessage string `json:"conversionMessage,omitempty"`
}

type platformPartSource struct {
	Platform  string `json:"platform,omitempty"`
	MessageID string `json:"messageID,omitempty"`
	FileID    string `json:"fileID,omitempty"`
}

type platformContextSender struct {
	Platform    string `json:"platform,omitempty"`
	SenderID    string `json:"senderID,omitempty"`
	UserID      string `json:"userID,omitempty"`
	Handle      string `json:"handle,omitempty"`
	Email       string `json:"email,omitempty"`
	Name        string `json:"name,omitempty"`
	CallingName string `json:"callingName,omitempty"`
}

type platformContextMessage struct {
	Speaker            string                    `json:"speaker"`
	SpeakerCallingName string                    `json:"speakerCallingName,omitempty"`
	SpeakerHandle      string                    `json:"speakerHandle,omitempty"`
	Text               string                    `json:"text"`
	SentAt             string                    `json:"sentAt,omitempty"`
	InputAttachments   []platformInputAttachment `json:"inputAttachments,omitempty"`
}

type platformHandle struct {
	Platform       string `json:"platform"`
	ConversationID string `json:"conversationID"`
	RootID         string `json:"rootID,omitempty"`
	MessageID      string `json:"messageID,omitempty"`
	TeamID         string `json:"teamID,omitempty"`
}

func encodePlatformHandle(handle platformHandle) (string, error) {
	document, errorValue := json.Marshal(handle)
	if errorValue != nil {
		return "", errorValue
	}
	return base64.RawURLEncoding.EncodeToString(document), nil
}

func decodePlatformHandle(value string) (platformHandle, error) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return platformHandle{}, errors.New("platform handle is empty")
	}
	document, errorValue := base64.RawURLEncoding.DecodeString(trimmedValue)
	if errorValue != nil {
		return platformHandle{}, errorValue
	}
	var handle platformHandle
	if errorValue := json.Unmarshal(document, &handle); errorValue != nil {
		return platformHandle{}, errorValue
	}
	return handle, nil
}

