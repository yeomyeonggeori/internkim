package mattermostinteractive

type Payload struct {
	UserID         string  `json:"user_id"`
	PostID         string  `json:"post_id"`
	ChannelID      string  `json:"channel_id"`
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
	ResponseLanguage string `json:"responseLanguage,omitempty"`
}

type Response struct {
	Update        any    `json:"update,omitempty"`
	EphemeralText string `json:"ephemeral_text,omitempty"`
	Error         *Error `json:"error,omitempty"`
}

type Error struct {
	Message string `json:"message"`
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
