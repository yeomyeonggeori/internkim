package mattermostinteractive

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const ActionPath = "/_internkim/mattermost/actions"

type Payload struct {
	UserID         string  `json:"user_id"`
	PostID         string  `json:"post_id"`
	ChannelID      string  `json:"channel_id"`
	RootID         string  `json:"root_id,omitempty"`
	TeamID         string  `json:"team_id"`
	SelectedOption string  `json:"selected_option,omitempty"`
	Context        Context `json:"context"`
}

type Context struct {
	Action           string `json:"action"`
	Token            string `json:"token"`
	LocationID       string `json:"locationID,omitempty"`
	InteractionID    string `json:"interactionID,omitempty"`
	TaskRunID        string `json:"taskRunID,omitempty"`
	ConversationID   string `json:"conversationID,omitempty"`
	ReplyTargetID    string `json:"replyTargetID,omitempty"`
	ChoiceKey        string `json:"choiceKey,omitempty"`
	ChoiceLabel      string `json:"choiceLabel,omitempty"`
	ResponseLanguage string `json:"responseLanguage,omitempty"`
	TargetUserID     string `json:"targetUserID,omitempty"`
}

type Response struct {
	Update        any    `json:"update,omitempty"`
	EphemeralText string `json:"ephemeral_text,omitempty"`
}

type Attachment struct {
	Fallback string   `json:"fallback"`
	Text     string   `json:"text,omitempty"`
	Actions  []Action `json:"actions"`
}

type Action struct {
	ID          string      `json:"id"`
	Type        string      `json:"type"`
	Name        string      `json:"name"`
	Tooltip     string      `json:"tooltip,omitempty"`
	Style       string      `json:"style,omitempty"`
	Options     []Option    `json:"options,omitempty"`
	Integration Integration `json:"integration"`
}

type Option struct {
	Text  string `json:"text"`
	Value string `json:"value"`
}

type Integration struct {
	URL     string  `json:"url"`
	Context Context `json:"context"`
}

type ActionBuilder struct {
	URL   string
	Token string
}

func ActionURL(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + ActionPath
}

func LoadOrCreateToken(path string, size int) (string, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", errors.New("mattermost interactive token path is required")
	}
	if token := readToken(trimmedPath); token != "" {
		return token, nil
	}
	if size <= 0 {
		return "", errors.New("mattermost interactive token size must be positive")
	}
	if errorValue := os.MkdirAll(filepath.Dir(trimmedPath), 0o700); errorValue != nil {
		return "", errorValue
	}
	token, errorValue := randomToken(size)
	if errorValue != nil {
		return "", errorValue
	}
	temporaryFile, errorValue := os.CreateTemp(filepath.Dir(trimmedPath), ".mattermost-interactive-token-*")
	if errorValue != nil {
		return "", errorValue
	}
	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)
	if errorValue := temporaryFile.Chmod(0o600); errorValue != nil {
		temporaryFile.Close()
		return "", errorValue
	}
	if _, errorValue := temporaryFile.WriteString(token + "\n"); errorValue != nil {
		temporaryFile.Close()
		return "", errorValue
	}
	if errorValue := temporaryFile.Close(); errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.Link(temporaryPath, trimmedPath); errorValue == nil {
		return token, nil
	} else if !errors.Is(errorValue, os.ErrExist) {
		return "", errorValue
	}
	if existingToken := readToken(trimmedPath); existingToken != "" {
		return existingToken, nil
	}
	return "", errors.New("mattermost interactive token is empty")
}

func readToken(path string) string {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return ""
	}
	return strings.TrimSpace(string(document))
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, errorValue := rand.Read(value); errorValue != nil {
		return "", errorValue
	}
	return hex.EncodeToString(value), nil
}

func (builder ActionBuilder) Button(id string, name string, tooltip string, style string, context Context) Action {
	return Button(id, name, tooltip, style, builder.URL, builder.context(id, context))
}

func (builder ActionBuilder) Select(id string, name string, context Context, options []Option) Action {
	return Select(id, name, builder.URL, builder.context(id, context), options)
}

func (builder ActionBuilder) context(actionID string, context Context) Context {
	if strings.TrimSpace(context.Action) == "" {
		context.Action = actionID
	}
	context.Token = builder.Token
	return context
}

func Button(id string, name string, tooltip string, style string, url string, context Context) Action {
	return Action{
		ID:      id,
		Type:    "button",
		Name:    name,
		Tooltip: tooltip,
		Style:   style,
		Integration: Integration{
			URL:     url,
			Context: context,
		},
	}
}

func Select(id string, name string, url string, context Context, options []Option) Action {
	return Action{
		ID:      id,
		Type:    "select",
		Name:    name,
		Options: options,
		Integration: Integration{
			URL:     url,
			Context: context,
		},
	}
}

func ClearAttachmentsUpdate() map[string]any {
	return map[string]any{"props": map[string]any{"attachments": []any{}}}
}
