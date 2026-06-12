package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mattermostEphemeralPluginID = "com.internkim.ephemeral"

type mattermostEphemeralPluginUpdateRequest struct {
	UserID    string `json:"userID"`
	PostID    string `json:"postID"`
	ChannelID string `json:"channelID"`
	RootID    string `json:"rootID,omitempty"`
	Message   string `json:"message"`
}

type ActionContext struct {
	ChoiceKey      string
	ChoiceLabel    string
	SelectedOption string
}

func (service *Service) ensureMattermostEphemeralPluginWithRetry(ctx context.Context) {
	for attempt := 0; attempt < 10; attempt++ {
		attemptContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, errorValue := service.ensureMattermostEphemeralPlugin(attemptContext)
		cancel()
		if errorValue == nil {
			return
		}
		log.Printf("Mattermost ephemeral plugin sync failed (attempt %d): %v", attempt+1, errorValue)
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
	}
}

func (service *Service) ensureMattermostEphemeralPlugin(ctx context.Context) (string, error) {
	secret, errorValue := service.ensureMattermostEphemeralPluginSecret()
	if errorValue != nil {
		return "", errorValue
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.uploadMattermostEphemeralPlugin(ctx, adminToken); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.enableMattermostEphemeralPlugin(ctx, adminToken); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.patchMattermostEphemeralPluginSecret(ctx, adminToken, secret); errorValue != nil {
		return "", errorValue
	}
	return secret, nil
}

func (service *Service) ensureMattermostEphemeralPluginSecret() (string, error) {
	path := service.mattermostEphemeralPluginSecretPath()
	secret := strings.TrimSpace(readTrimmedFile(path))
	if secret != "" {
		return secret, nil
	}
	secret = randomHex(32)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.WriteFile(path, []byte(secret+"\n"), 0o600); errorValue != nil {
		return "", errorValue
	}
	return secret, nil
}

func (service *Service) mattermostEphemeralPluginSecret() string {
	return strings.TrimSpace(readTrimmedFile(service.mattermostEphemeralPluginSecretPath()))
}

func (service *Service) mattermostEphemeralPluginSecretPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "mattermost-ephemeral-plugin-secret")
}

func (service *Service) uploadMattermostEphemeralPlugin(ctx context.Context, token string) error {
	body, contentType, errorValue := mattermostPluginUploadBody(service.Configuration.MattermostPluginBundlePath)
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/plugins?force=true"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", contentType)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostStatusError(response)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func mattermostPluginUploadBody(bundlePath string) (io.Reader, string, error) {
	document, errorValue := os.ReadFile(bundlePath)
	if errorValue != nil {
		return nil, "", errorValue
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, errorValue := writer.CreateFormFile("plugin", filepath.Base(bundlePath))
	if errorValue != nil {
		return nil, "", errorValue
	}
	if _, errorValue := part.Write(document); errorValue != nil {
		return nil, "", errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return nil, "", errorValue
	}
	return bytes.NewReader(body.Bytes()), writer.FormDataContentType(), nil
}

func (service *Service) enableMattermostEphemeralPlugin(ctx context.Context, token string) error {
	path := "/api/v4/plugins/" + url.PathEscape(mattermostEphemeralPluginID) + "/enable"
	return service.mattermostRequest(ctx, http.MethodPost, path, token, nil, nil)
}

func (service *Service) patchMattermostEphemeralPluginSecret(ctx context.Context, token string, secret string) error {
	body := map[string]any{
		"PluginSettings": map[string]any{
			"Plugins": map[string]any{
				mattermostEphemeralPluginID: map[string]string{"secret": secret},
			},
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) updateMattermostAskEphemeralPostInBackground(payload mattermostInteractivePayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if errorValue := service.updateMattermostAskEphemeralPost(ctx, payload); errorValue != nil {
		log.Printf("mattermost ask ephemeral update failed: %v", errorValue)
	}
}

func (service *Service) updateMattermostAskEphemeralPost(ctx context.Context, payload mattermostInteractivePayload) error {
	secret := service.mattermostEphemeralPluginSecret()
	if secret == "" {
		return nil
	}
	updateRequest := mattermostEphemeralPluginUpdateRequest{
		UserID:    strings.TrimSpace(payload.UserID),
		PostID:    strings.TrimSpace(payload.PostID),
		ChannelID: strings.TrimSpace(payload.ChannelID),
		RootID:    strings.TrimSpace(payload.RootID),
		Message:   buildAskResolutionMessage(payload.Context.Action, actionContextFromMattermostPayload(payload)),
	}
	if updateRequest.UserID == "" || updateRequest.PostID == "" || updateRequest.ChannelID == "" || updateRequest.Message == "" {
		return nil
	}
	document, errorValue := json.Marshal(updateRequest)
	if errorValue != nil {
		return errorValue
	}
	endpoint := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/plugins/" + mattermostEphemeralPluginID + "/api/v1/update-ephemeral"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-InternKim-Token", secret)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostStatusError(response)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func actionContextFromMattermostPayload(payload mattermostInteractivePayload) ActionContext {
	return ActionContext{
		ChoiceKey:      strings.TrimSpace(payload.Context.ChoiceKey),
		ChoiceLabel:    strings.TrimSpace(payload.Context.ChoiceLabel),
		SelectedOption: strings.TrimSpace(payload.SelectedOption),
	}
}

func buildAskResolutionMessage(actionType string, context ActionContext) string {
	switch strings.TrimSpace(actionType) {
	case "ask.confirm":
		return "확인"
	case "ask.cancel":
		return "취소"
	case "ask.choice":
		return firstNonEmpty(context.ChoiceLabel, mattermostSelectedChoiceLabel(context.SelectedOption), mattermostSelectedChoiceKey(context.SelectedOption), context.ChoiceKey)
	default:
		return strings.TrimSpace(actionType)
	}
}

func mattermostSelectedChoiceLabel(selectedOption string) string {
	selectedChoice, isFound := parseMattermostSelectedChoice(selectedOption)
	if isFound {
		return selectedChoice.Label
	}
	return ""
}
