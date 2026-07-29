package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type passwordLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (service *Service) handleMattermostPasswordLogin(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.NotFound(responseWriter, request)
		return
	}
	var payload passwordLoginRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, "invalid request", http.StatusBadRequest)
		return
	}
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	if email == "" || payload.Password == "" {
		http.Error(responseWriter, "email and password are required", http.StatusBadRequest)
		return
	}
	userRecord, errorValue := service.mattermostPasswordAuthenticate(request.Context(), email, payload.Password)
	if errorValue != nil {
		http.Error(responseWriter, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if userRecord.IsBot || userRecord.DeleteAt != 0 {
		http.Error(responseWriter, "invalid email or password", http.StatusUnauthorized)
		return
	}
	if !service.isFlowStaffActor(request.Context(), email) {
		http.Error(responseWriter, "account not invited", http.StatusForbidden)
		return
	}
	if errorValue := service.issueWebSessionCookie(responseWriter, request, mattermostUserRecord{Email: email}); errorValue != nil {
		http.Error(responseWriter, "session_failed", http.StatusInternalServerError)
		return
	}
	logAuditEvent("mattermost password login success")
	go service.ensureUserChannelMembership(context.Background(), email)
	service.writeJSON(responseWriter, map[string]string{"ok": "true", "secretHex": service.buzzSecretForEmail(request.Context(), email)})
}

func (service *Service) buzzSecretForEmail(ctx context.Context, email string) string {
	seed := service.buzzKeySeed()
	if seed == "" {
		return ""
	}
	subject := service.buzzVaultSubject(ctx, email)
	version := service.buzzIdentityVersion(subject)
	return buzzKeyForVersion(seed, email, version)
}

// handleAuthIdentity hands an already-signed-in session its own deterministic
// Buzz key so the browser can establish the messaging identity without a
// separate vault-setup dialog. The session is the gate; the key is a function
// of the session email, so this exposes nothing the account does not already own.
func (service *Service) handleAuthIdentity(responseWriter http.ResponseWriter, request *http.Request) {
	email := service.webActorEmail(request)
	if email == "" {
		http.Error(responseWriter, "unauthorized", http.StatusUnauthorized)
		return
	}
	secretHex := service.buzzSecretForEmail(request.Context(), email)
	if secretHex == "" {
		http.Error(responseWriter, "buzz identity unavailable", http.StatusNotImplemented)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"secretHex": secretHex})
}

func (service *Service) mattermostPasswordAuthenticate(ctx context.Context, email string, password string) (mattermostUserRecord, error) {
	requestBody, errorValue := json.Marshal(map[string]string{"login_id": email, "password": password})
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/users/login"
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostUserRecord{}, mattermostStatusError(response)
	}
	var userRecord mattermostUserRecord
	if errorValue := json.NewDecoder(response.Body).Decode(&userRecord); errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	return userRecord, nil
}
