package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
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

func (service Service) createCompanionPairingForMattermostSenderRecord(ctx context.Context, sender platformContextSender) (companionConnectPairingResponse, error) {
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

func (service Service) mattermostReplyMessageWithRecovery(ctx context.Context, handle platformHandle, request replyRequest) (string, error) {
	recoveryAction, hasRecoveryAction := companionConnectRecoveryAction(request.RecoveryActions)
	if !hasRecoveryAction {
		return request.Message, nil
	}
	if isMattermostDirectHandle(handle) {
		recoveryMessage, errorValue := service.mattermostCompanionRecoveryMessage(ctx, recoveryAction)
		if errorValue != nil {
			return "", errorValue
		}
		return joinMattermostMessages(request.Message, recoveryMessage), nil
	}
	if strings.TrimSpace(recoveryAction.PlatformUserID) == "" {
		return joinMattermostMessages(request.Message, "Companion 연결이 필요합니다. `/connect`를 실행해 주세요."), nil
	}
	recoveryMessage, errorValue := service.mattermostCompanionRecoveryMessage(ctx, recoveryAction)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.sendMattermostDirectMessage(ctx, recoveryAction.PlatformUserID, recoveryMessage); errorValue != nil {
		return "", errorValue
	}
	return "Companion 연결 안내를 DM으로 보냈어요.", nil
}

func companionConnectRecoveryAction(recoveryActions []capabilities.RecoveryAction) (capabilities.RecoveryAction, bool) {
	for _, recoveryAction := range recoveryActions {
		if recoveryAction.Kind == "companion_connect" {
			return recoveryAction, true
		}
	}
	return capabilities.RecoveryAction{}, false
}

func (service Service) mattermostCompanionRecoveryMessage(ctx context.Context, recoveryAction capabilities.RecoveryAction) (string, error) {
	response, errorValue := service.createCompanionPairingForMattermostSenderRecord(ctx, platformContextSender{
		Platform: "mattermost",
		SenderID: recoveryAction.PlatformUserID,
		UserID:   recoveryAction.PlatformUserID,
	})
	if errorValue != nil {
		return "", errorValue
	}
	return companionConnectRecoveryMessage(response, recoveryAction), nil
}

func companionConnectRecoveryMessage(response companionConnectPairingResponse, recoveryAction capabilities.RecoveryAction) string {
	downloadURL := firstNonEmpty(recoveryAction.DownloadURL, capabilities.CompanionMacOSBetaDownloadURL())
	connectCommand := firstNonEmpty(recoveryAction.ConnectCommand, "/connect")
	return "Companion 브라우저가 필요한 작업입니다.\n" +
		"Companion 앱 다운로드: " + downloadURL + "\n" +
		"Companion 연결 코드: `" + response.Code + "`\n" +
		"Companion 앱에서 이 링크를 열거나 코드를 입력하세요.\n" +
		"[Companion 앱 열기](" + response.DeepLink + ")\n" +
		"다시 연결이 필요하면 `" + connectCommand + "`를 실행하세요.\n" +
		"만료: " + response.ExpiresAt.Local().Format("15:04")
}

func isMattermostDirectHandle(handle platformHandle) bool {
	return strings.EqualFold(strings.TrimSpace(handle.ChannelType), "D") || strings.HasPrefix(strings.TrimSpace(handle.ConversationID), "dm:")
}

func joinMattermostMessages(messages ...string) string {
	parts := []string{}
	for _, message := range messages {
		trimmedMessage := strings.TrimSpace(message)
		if trimmedMessage != "" {
			parts = append(parts, trimmedMessage)
		}
	}
	return strings.Join(parts, "\n\n")
}

func (service Service) companionConnectDeviceURL() string {
	admindBaseURL := strings.TrimRight(strings.TrimSpace(service.Configuration.AdmindBaseURL), "/")
	if admindBaseURL != "" && !isLocalBaseURL(admindBaseURL) {
		return admindBaseURL
	}
	fleetID := strings.ToLower(strings.TrimSpace(readSecretValue(service.Configuration.FleetIDPath)))
	if fleetID == "" {
		return admindBaseURL
	}
	return "https://" + fleetID + ".intern.kim"
}

func isLocalBaseURL(value string) bool {
	return strings.Contains(value, "127.0.0.1") || strings.Contains(value, "localhost") || strings.Contains(value, "[::1]")
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
