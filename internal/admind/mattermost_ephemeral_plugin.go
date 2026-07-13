package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

const mattermostEphemeralPluginID = "com.internkim.ephemeral"

type mattermostEphemeralPluginDeleteRequest struct {
	UserID string `json:"userID"`
	PostID string `json:"postID"`
}

func (service *Service) ensureMattermostEphemeralPluginWithRetry(ctx context.Context) {
	for attempt := 0; attempt < 10; attempt++ {
		attemptContext, cancel := context.WithTimeout(ctx, 3*time.Minute)
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
	if errorValue := service.enableMattermostPluginUploads(ctx, adminToken); errorValue != nil {
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
	service.logMattermostEphemeralPluginStatus(ctx, adminToken)
	return secret, nil
}

func (service *Service) writeMattermostPluginSyncDiagnostic(responseWriter http.ResponseWriter, request *http.Request) {
	_, errorValue := service.ensureMattermostEphemeralPlugin(request.Context())
	if errorValue != nil {
		service.writeJSON(responseWriter, map[string]string{"status": "failed", "error": errorValue.Error()})
		return
	}
	probeStatus, probeError := service.postMattermostEphemeralPluginDelete(request.Context(), service.mattermostEphemeralPluginSecret(), []byte(`{}`))
	probe := map[string]any{"status": "ok", "authProbeStatus": probeStatus}
	if probeError != nil {
		probe["authProbeError"] = probeError.Error()
	}
	service.writeJSON(responseWriter, probe)
}

func (service *Service) logMattermostEphemeralPluginStatus(ctx context.Context, token string) {
	var plugins struct {
		Active   []struct{ ID, Version string } `json:"active"`
		Inactive []struct{ ID, Version string } `json:"inactive"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/plugins", token, nil, &plugins); errorValue != nil {
		log.Printf("Mattermost ephemeral plugin status lookup failed: %v", errorValue)
		return
	}
	log.Printf("Mattermost plugins after ephemeral sync: active=%v inactive=%v", plugins.Active, plugins.Inactive)
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
		statusError := mattermostStatusError(response)
		if strings.Contains(statusError.Error(), "install_id") {
			return service.reinstallMattermostEphemeralPlugin(ctx, token)
		}
		return statusError
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func (service *Service) reinstallMattermostEphemeralPlugin(ctx context.Context, token string) error {
	removePath := "/api/v4/plugins/" + url.PathEscape(mattermostEphemeralPluginID)
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, removePath, token, nil, nil); errorValue != nil {
		return errorValue
	}
	return service.uploadMattermostEphemeralPlugin(ctx, token)
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

func (service *Service) enableMattermostPluginUploads(ctx context.Context, token string) error {
	enabled, errorValue := service.mattermostPluginUploadsEnabled(ctx, token)
	if errorValue != nil || enabled {
		return errorValue
	}
	body := map[string]any{
		"PluginSettings": map[string]any{
			"Enable":                 true,
			"EnableUploads":          true,
			"RequirePluginSignature": false,
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) mattermostPluginUploadsEnabled(ctx context.Context, token string) (bool, error) {
	var configuration struct {
		PluginSettings struct {
			Enable        *bool `json:"Enable"`
			EnableUploads *bool `json:"EnableUploads"`
		} `json:"PluginSettings"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/config", token, nil, &configuration); errorValue != nil {
		return false, errorValue
	}
	settings := configuration.PluginSettings
	return settings.Enable != nil && *settings.Enable && settings.EnableUploads != nil && *settings.EnableUploads, nil
}

func (service *Service) waitForMattermostReady(ctx context.Context) error {
	endpoint := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/system/ping"
	for {
		request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if errorValue != nil {
			return errorValue
		}
		response, errorValue := service.httpClient().Do(request)
		if errorValue == nil {
			statusCode := response.StatusCode
			response.Body.Close()
			if statusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func (service *Service) enableMattermostEphemeralPlugin(ctx context.Context, token string) error {
	path := "/api/v4/plugins/" + url.PathEscape(mattermostEphemeralPluginID) + "/enable"
	return service.mattermostRequest(ctx, http.MethodPost, path, token, nil, nil)
}

func (service *Service) patchMattermostEphemeralPluginSecret(ctx context.Context, token string, secret string) error {
	body := map[string]any{
		"PluginSettings": map[string]any{
			"Plugins": map[string]any{
				mattermostEphemeralPluginID: map[string]string{
					"secret":           secret,
					"runtimeHealthURL": blueclaw.BlueclawHealthCheckURL(),
					"botUsername":      "internkim",
				},
			},
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) deleteMattermostAskEphemeralPostInBackground(payload mattermostInteractivePayload) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if errorValue := service.deleteMattermostAskEphemeralPost(ctx, payload); errorValue != nil {
		log.Printf("mattermost ask ephemeral delete failed: %v", errorValue)
	}
}

func (service *Service) deleteMattermostAskEphemeralPost(ctx context.Context, payload mattermostInteractivePayload) error {
	secret := service.mattermostEphemeralPluginSecret()
	if secret == "" {
		return fmt.Errorf("mattermost ephemeral plugin secret is not configured; deploy the mattermostPlugins component or run sync-mattermost-plugins")
	}
	deleteRequest := mattermostEphemeralPluginDeleteRequest{
		UserID: strings.TrimSpace(payload.UserID),
		PostID: strings.TrimSpace(payload.PostID),
	}
	if deleteRequest.UserID == "" || deleteRequest.PostID == "" {
		return nil
	}
	document, errorValue := json.Marshal(deleteRequest)
	if errorValue != nil {
		return errorValue
	}
	statusCode, errorValue := service.postMattermostEphemeralPluginDelete(ctx, secret, document)
	if statusCode != http.StatusNotFound {
		return errorValue
	}
	installedSecret, ensureError := service.ensureMattermostEphemeralPlugin(ctx)
	if ensureError != nil {
		return ensureError
	}
	_, errorValue = service.postMattermostEphemeralPluginDelete(ctx, installedSecret, document)
	return errorValue
}

func (service *Service) postMattermostEphemeralPluginDelete(ctx context.Context, secret string, document []byte) (int, error) {
	endpoint := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/plugins/" + mattermostEphemeralPluginID + "/api/v1/delete-ephemeral"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return 0, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-InternKim-Token", secret)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return 0, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return response.StatusCode, mattermostStatusError(response)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return response.StatusCode, nil
}
