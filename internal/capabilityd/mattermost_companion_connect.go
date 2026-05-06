package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

type companionConnectPairingRequest struct {
	OwnerPlatform       string `json:"ownerPlatform,omitempty"`
	OwnerPlatformUserID string `json:"ownerPlatformUserID,omitempty"`
	OwnerEmail          string `json:"ownerEmail,omitempty"`
	OwnerName           string `json:"ownerName,omitempty"`
	DeviceURL           string `json:"deviceURL,omitempty"`
}

type companionConnectPairingResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
	DeepLink  string    `json:"deepLink"`
}

func (service Service) handleMattermostCompanionConnectCommand(ctx context.Context, event platformInboundEvent) (bool, error) {
	if !isMattermostCompanionConnectCommand(event) {
		return false, nil
	}
	response, errorValue := service.createCompanionPairingForMattermostSender(ctx, event)
	if errorValue != nil {
		return true, errorValue
	}
	message := companionConnectMessage(response)
	if strings.EqualFold(strings.TrimSpace(event.Context.ConversationType), "D") {
		return true, service.postMattermostMessage(ctx, event.Context.ChannelID, "", message)
	}
	if errorValue := service.sendMattermostDirectMessage(ctx, event.SenderID, message); errorValue != nil {
		return true, errorValue
	}
	return true, service.postMattermostMessage(ctx, event.Context.ChannelID, "", "Companion 연결 링크를 DM으로 보냈어요.")
}

func isMattermostCompanionConnectCommand(event platformInboundEvent) bool {
	prompt := strings.ToLower(strings.TrimSpace(event.Prompt))
	if prompt == "/internkim connect" || prompt == "companion connect" {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(event.Context.ConversationType), "D") {
		return false
	}
	return prompt == "connect" || prompt == "컴패니언 연결"
}

func (service Service) createCompanionPairingForMattermostSender(ctx context.Context, event platformInboundEvent) (companionConnectPairingResponse, error) {
	sender := event.Context.Sender
	if strings.TrimSpace(sender.UserID) == "" && strings.TrimSpace(sender.Email) == "" {
		return companionConnectPairingResponse{}, errors.New("mattermost sender identity is missing")
	}
	payload := companionConnectPairingRequest{
		OwnerPlatform:       "mattermost",
		OwnerPlatformUserID: firstNonEmpty(sender.UserID, sender.SenderID),
		OwnerEmail:          strings.ToLower(strings.TrimSpace(sender.Email)),
		OwnerName:           strings.TrimSpace(sender.Name),
		DeviceURL:           service.companionConnectDeviceURL(),
	}
	var response companionConnectPairingResponse
	if errorValue := service.postAdmindJSON(ctx, "/_internkim/companion/pairing-codes", payload, &response); errorValue != nil {
		return companionConnectPairingResponse{}, errorValue
	}
	return response, nil
}

func (service Service) companionConnectDeviceURL() string {
	admindBaseURL := strings.TrimRight(strings.TrimSpace(service.Configuration.AdmindBaseURL), "/")
	if admindBaseURL != "" && !isLocalBaseURL(admindBaseURL) {
		return admindBaseURL
	}
	deviceID := strings.ToLower(strings.TrimSpace(readSecretValue(service.Configuration.DeviceIDPath)))
	if deviceID == "" {
		return admindBaseURL
	}
	return "https://" + deviceID + ".example.test"
}

func isLocalBaseURL(value string) bool {
	return strings.Contains(value, "127.0.0.1") || strings.Contains(value, "localhost") || strings.Contains(value, "[::1]")
}

func companionConnectMessage(response companionConnectPairingResponse) string {
	expiresAt := response.ExpiresAt.Local().Format("15:04")
	return "Companion 연결 코드: `" + response.Code + "`\n" +
		"Companion 앱에서 이 링크를 열거나 코드를 입력하세요.\n" +
		response.DeepLink + "\n" +
		"만료: " + expiresAt
}

func (service Service) postAdmindJSON(ctx context.Context, path string, requestDocument any, responseDocument any) error {
	document, errorValue := json.Marshal(requestDocument)
	if errorValue != nil {
		return errorValue
	}
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + path
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		return errors.New(response.Status)
	}
	return json.NewDecoder(response.Body).Decode(responseDocument)
}

func (service Service) sendMattermostDirectMessage(ctx context.Context, userID string, message string) error {
	botUserID, errorValue := service.mattermostBotUserID(ctx)
	if errorValue != nil {
		return errorValue
	}
	var channel struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/direct", []string{userID, botUserID}, &channel); errorValue != nil {
		return errorValue
	}
	return service.postMattermostMessage(ctx, channel.ID, "", message)
}

func (service Service) mattermostBotUserID(ctx context.Context) (string, error) {
	var response struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", nil, &response); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(response.ID) == "" {
		return "", errors.New("mattermost bot user id is missing")
	}
	return response.ID, nil
}

func (service Service) postMattermostMessage(ctx context.Context, channelID string, rootID string, message string) error {
	body := map[string]any{
		"channel_id": channelID,
		"message":    message,
	}
	if strings.TrimSpace(rootID) != "" {
		body["root_id"] = rootID
	}
	var response struct {
		ID string `json:"id"`
	}
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", body, &response)
}
