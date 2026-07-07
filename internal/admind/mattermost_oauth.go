package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	mattermostOAuthStartPath    = "/auth/mattermost/start"
	mattermostOAuthCallbackPath = "/auth/mattermost/callback"
	mattermostOAuthStateTTL     = 10 * time.Minute
	mattermostOAuthAppName      = "InternKim Web"
)

type mattermostOAuthClientFile struct {
	AppID        string `json:"app_id,omitempty"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	CallbackURL  string `json:"callback_url"`
	Homepage     string `json:"homepage"`
}

type mattermostOAuthStateRecord struct {
	CreatedAt   time.Time
	RedirectURI string
	ReturnPath  string
}

type mattermostOAuthAppResponse struct {
	ID           string   `json:"id"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"client_secret"`
	CallbackURLs []string `json:"callback_urls"`
	Homepage     string   `json:"homepage"`
	IsTrusted    *bool    `json:"is_trusted,omitempty"`
}

func (service *Service) handleMattermostOAuthStart(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	service.cleanupExpiredMattermostOAuthStates(time.Now())
	returnPath := safeWebReturnPath(request.URL.Query().Get("return"))
	if returnPath == "" {
		respondMattermostAuthError(responseWriter, http.StatusBadRequest, "잘못된 이동 경로입니다.")
		logAuditEvent("mattermost oauth start denied: unsafe return")
		return
	}
	redirectURI := service.mattermostOAuthRedirectURI(request)
	if errorValue := service.ensureMattermostWebOAuthAppForRedirectURI(request.Context(), redirectURI); errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusServiceUnavailable, "Mattermost 로그인 앱을 준비하지 못했습니다.")
		logAuditEvent("mattermost oauth start failed: client")
		return
	}
	configuration, errorValue := service.buildMattermostOAuthConfig(redirectURI)
	if errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusServiceUnavailable, "Mattermost 로그인이 아직 설정되지 않았습니다.")
		logAuditEvent("mattermost oauth start failed: configuration")
		return
	}
	state, errorValue := generateRandomURLToken(32)
	if errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusInternalServerError, "로그인 상태값을 만들지 못했습니다.")
		logAuditEvent("mattermost oauth start failed: state")
		return
	}
	service.webOAuthStates.Store(state, &mattermostOAuthStateRecord{
		CreatedAt:   time.Now().UTC(),
		RedirectURI: redirectURI,
		ReturnPath:  returnPath,
	})
	http.Redirect(responseWriter, request, configuration.AuthCodeURL(state), http.StatusFound)
	logAuditEvent("mattermost oauth start")
}

func (service *Service) handleMattermostOAuthCallback(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	service.cleanupExpiredMattermostOAuthStates(time.Now())
	state, code, ok := readMattermostOAuthCallbackParams(responseWriter, request)
	if !ok {
		return
	}
	record, ok := service.consumeMattermostOAuthState(responseWriter, state)
	if !ok {
		return
	}
	userRecord, ok := service.completeMattermostOAuthExchange(responseWriter, request, record, code)
	if !ok {
		return
	}
	if !service.canAuthenticateWebReturnPath(request.Context(), userRecord.Email, record.ReturnPath) {
		respondMattermostAuthError(responseWriter, http.StatusForbidden, "InternKim 사용 권한이 없습니다.")
		logAuditEvent("mattermost oauth callback denied: non_staff")
		return
	}
	if errorValue := service.issueWebSessionCookie(responseWriter, request, userRecord); errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusInternalServerError, "웹 세션을 만들지 못했습니다.")
		logAuditEvent("mattermost oauth callback failed: session")
		return
	}
	http.Redirect(responseWriter, request, record.ReturnPath, http.StatusFound)
	logAuditEvent("mattermost oauth callback success")
}

func readMattermostOAuthCallbackParams(responseWriter http.ResponseWriter, request *http.Request) (string, string, bool) {
	query := request.URL.Query()
	if callbackError := strings.TrimSpace(query.Get("error")); callbackError != "" {
		respondMattermostAuthError(responseWriter, http.StatusBadRequest, "Mattermost 로그인이 취소되었거나 거부되었습니다.")
		logAuditEvent("mattermost oauth callback returned error")
		return "", "", false
	}
	state := strings.TrimSpace(query.Get("state"))
	code := strings.TrimSpace(query.Get("code"))
	if state == "" || code == "" {
		respondMattermostAuthError(responseWriter, http.StatusBadRequest, "Mattermost 로그인 응답이 올바르지 않습니다.")
		logAuditEvent("mattermost oauth callback denied: missing params")
		return "", "", false
	}
	return state, code, true
}

func (service *Service) consumeMattermostOAuthState(responseWriter http.ResponseWriter, state string) (*mattermostOAuthStateRecord, bool) {
	recordRaw, found := service.webOAuthStates.LoadAndDelete(state)
	if !found {
		respondMattermostAuthError(responseWriter, http.StatusBadRequest, "로그인 상태가 만료되었습니다. 다시 시도해주세요.")
		logAuditEvent("mattermost oauth callback denied: unknown state")
		return nil, false
	}
	record, ok := recordRaw.(*mattermostOAuthStateRecord)
	if !ok || time.Since(record.CreatedAt) > mattermostOAuthStateTTL {
		respondMattermostAuthError(responseWriter, http.StatusBadRequest, "로그인 상태가 만료되었습니다. 다시 시도해주세요.")
		logAuditEvent("mattermost oauth callback denied: expired state")
		return nil, false
	}
	return record, true
}

func (service *Service) completeMattermostOAuthExchange(responseWriter http.ResponseWriter, request *http.Request, record *mattermostOAuthStateRecord, code string) (mattermostUserRecord, bool) {
	configuration, errorValue := service.buildMattermostOAuthConfig(record.RedirectURI)
	if errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusServiceUnavailable, "Mattermost 로그인 설정을 불러오지 못했습니다.")
		logAuditEvent("mattermost oauth callback failed: configuration")
		return mattermostUserRecord{}, false
	}
	ctx := context.WithValue(request.Context(), oauth2.HTTPClient, service.httpClient())
	token, errorValue := configuration.Exchange(ctx, code)
	if errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusBadGateway, "Mattermost 로그인 토큰을 확인하지 못했습니다.")
		logAuditEvent("mattermost oauth callback failed: exchange")
		return mattermostUserRecord{}, false
	}
	userRecord, errorValue := service.fetchMattermostOAuthUser(request.Context(), token)
	if errorValue != nil {
		respondMattermostAuthError(responseWriter, http.StatusBadGateway, "Mattermost 사용자 정보를 확인하지 못했습니다.")
		logAuditEvent("mattermost oauth callback failed: userinfo")
		return mattermostUserRecord{}, false
	}
	return userRecord, true
}

func (service *Service) buildMattermostOAuthConfig(redirectURI string) (*oauth2.Config, error) {
	homepage := mattermostOAuthHomepageFromRedirectURI(redirectURI)
	clientFile, errorValue := service.loadMattermostOAuthClientForHomepage(homepage)
	if errorValue != nil {
		return nil, errorValue
	}
	internalBaseURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/")
	if internalBaseURL == "" {
		return nil, errors.New("mattermost internal base url is not configured")
	}
	publicBaseURL := strings.TrimRight(strings.TrimSpace(service.Configuration.MattermostPublicURL), "/")
	if publicBaseURL == "" {
		publicBaseURL = homepage
	}
	if publicBaseURL == "" {
		return nil, errors.New("mattermost public base url is not configured")
	}
	return &oauth2.Config{
		ClientID:     clientFile.ClientID,
		ClientSecret: clientFile.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  publicBaseURL + "/oauth/authorize",
			TokenURL: internalBaseURL + "/oauth/access_token",
		},
		RedirectURL: redirectURI,
	}, nil
}

func (service *Service) mattermostOAuthRedirectURI(request *http.Request) string {
	if !isLocalRequest(request) {
		return webAuthBaseURLFromRequest(request) + mattermostOAuthCallbackPath
	}
	publicBaseURL := strings.TrimRight(strings.TrimSpace(service.mattermostPublicBaseURL()), "/")
	if publicBaseURL != "" {
		return publicBaseURL + mattermostOAuthCallbackPath
	}
	return webAuthBaseURLFromRequest(request) + mattermostOAuthCallbackPath
}

func (service *Service) fetchMattermostOAuthUser(ctx context.Context, token *oauth2.Token) (mattermostUserRecord, error) {
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/users/me"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	token.SetAuthHeader(request)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return mattermostUserRecord{}, fmt.Errorf("mattermost users/me returned %d", response.StatusCode)
	}
	var userRecord mattermostUserRecord
	if errorValue := json.NewDecoder(response.Body).Decode(&userRecord); errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	if strings.TrimSpace(userRecord.ID) == "" || strings.TrimSpace(userRecord.Email) == "" {
		return mattermostUserRecord{}, errors.New("mattermost users/me response missing identity")
	}
	return userRecord, nil
}

func (service *Service) cleanupExpiredMattermostOAuthStates(now time.Time) {
	service.webOAuthStates.Range(func(key, value any) bool {
		record, ok := value.(*mattermostOAuthStateRecord)
		if !ok || now.Sub(record.CreatedAt) > mattermostOAuthStateTTL {
			service.webOAuthStates.Delete(key)
		}
		return true
	})
}

func (service *Service) loadMattermostOAuthClient() (mattermostOAuthClientFile, error) {
	return service.loadMattermostOAuthClientFromPath(service.Configuration.MattermostOAuthClientPath)
}

func (service *Service) loadMattermostOAuthClientForHomepage(homepage string) (mattermostOAuthClientFile, error) {
	return service.loadMattermostOAuthClientFromPath(service.mattermostOAuthClientPathForHomepage(homepage))
}

func (service *Service) loadMattermostOAuthClientFromPath(path string) (mattermostOAuthClientFile, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return mattermostOAuthClientFile{}, errorValue
	}
	var clientFile mattermostOAuthClientFile
	if errorValue := json.Unmarshal(document, &clientFile); errorValue != nil {
		return mattermostOAuthClientFile{}, errorValue
	}
	if strings.TrimSpace(clientFile.ClientID) == "" || strings.TrimSpace(clientFile.ClientSecret) == "" {
		return mattermostOAuthClientFile{}, errors.New("mattermost oauth client file missing credentials")
	}
	return clientFile, nil
}

func (service *Service) writeMattermostOAuthClient(clientFile mattermostOAuthClientFile) error {
	return service.writeMattermostOAuthClientForHomepage(clientFile.Homepage, clientFile)
}

func (service *Service) writeMattermostOAuthClientForHomepage(homepage string, clientFile mattermostOAuthClientFile) error {
	path := service.mattermostOAuthClientPathForHomepage(homepage)
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.MarshalIndent(clientFile, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, document, 0o600)
}

func (service *Service) ensureMattermostWebOAuthApp(ctx context.Context, token string) error {
	homepage := strings.TrimRight(strings.TrimSpace(service.mattermostPublicBaseURL()), "/")
	return service.ensureMattermostWebOAuthAppForHomepage(ctx, token, homepage)
}

func (service *Service) ensureMattermostWebOAuthAppForRedirectURI(ctx context.Context, redirectURI string) error {
	homepage := mattermostOAuthHomepageFromRedirectURI(redirectURI)
	if homepage == "" {
		return errors.New("mattermost oauth redirect uri is missing host")
	}
	clientFile, errorValue := service.loadMattermostOAuthClientForHomepage(homepage)
	if errorValue == nil && clientFile.CallbackURL == redirectURI && clientFile.Homepage == homepage {
		token, tokenError := service.mattermostAdminToken(ctx)
		if tokenError != nil {
			return nil
		}
		isCurrent, currentError := service.mattermostOAuthClientIsCurrent(ctx, token, clientFile, homepage, redirectURI)
		if currentError != nil {
			return currentError
		}
		if isCurrent {
			return nil
		}
		return service.createMattermostWebOAuthApp(ctx, token, homepage, redirectURI)
	}
	token, tokenError := service.mattermostAdminToken(ctx)
	if tokenError != nil {
		return tokenError
	}
	return service.ensureMattermostWebOAuthAppForHomepage(ctx, token, homepage)
}

func (service *Service) ensureMattermostWebOAuthAppForHomepage(ctx context.Context, token string, homepage string) error {
	homepage = strings.TrimRight(strings.TrimSpace(homepage), "/")
	if homepage == "" {
		return nil
	}
	callbackURL := homepage + mattermostOAuthCallbackPath
	if clientFile, errorValue := service.loadMattermostOAuthClient(); errorValue == nil {
		isCurrent, errorValue := service.mattermostOAuthClientIsCurrent(ctx, token, clientFile, homepage, callbackURL)
		if errorValue != nil {
			return errorValue
		}
		if isCurrent {
			return nil
		}
	}
	return service.createMattermostWebOAuthApp(ctx, token, homepage, callbackURL)
}

func (service *Service) mattermostOAuthClientIsCurrent(ctx context.Context, token string, clientFile mattermostOAuthClientFile, homepage string, callbackURL string) (bool, error) {
	if strings.TrimSpace(clientFile.AppID) == "" || clientFile.CallbackURL != callbackURL || clientFile.Homepage != homepage {
		return false, nil
	}
	var response mattermostOAuthAppResponse
	path := "/api/v4/oauth/apps/" + url.PathEscape(clientFile.AppID)
	errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &response)
	if isMattermostNotFound(errorValue) {
		return false, nil
	}
	if errorValue != nil {
		return false, errorValue
	}
	responseClientID := firstNonEmpty(response.ClientID, response.ID)
	if responseClientID != clientFile.ClientID {
		return false, nil
	}
	if response.Homepage != "" && strings.TrimRight(response.Homepage, "/") != homepage {
		return false, nil
	}
	if response.IsTrusted != nil && !*response.IsTrusted {
		return false, nil
	}
	return containsString(response.CallbackURLs, callbackURL), nil
}

func (service *Service) createMattermostWebOAuthApp(ctx context.Context, token string, homepage string, callbackURL string) error {
	body := map[string]any{
		"name":          mattermostOAuthAppName,
		"description":   "InternKim web app login",
		"homepage":      homepage,
		"callback_urls": []string{callbackURL},
		"is_trusted":    true,
	}
	var response mattermostOAuthAppResponse
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/oauth/apps", token, body, &response); errorValue != nil {
		return errorValue
	}
	clientID := firstNonEmpty(response.ClientID, response.ID)
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(response.ClientSecret) == "" {
		return errors.New("mattermost oauth app response missing credentials")
	}
	return service.writeMattermostOAuthClientForHomepage(homepage, mattermostOAuthClientFile{
		AppID:        response.ID,
		ClientID:     clientID,
		ClientSecret: response.ClientSecret,
		CallbackURL:  callbackURL,
		Homepage:     homepage,
	})
}

func (service *Service) mattermostPublicBaseURL() string {
	return service.mattermostFlowBaseURL()
}

func (service *Service) mattermostOAuthClientPathForHomepage(homepage string) string {
	basePath := service.Configuration.MattermostOAuthClientPath
	if strings.TrimSpace(basePath) == "" {
		return basePath
	}
	defaultHomepage := strings.TrimRight(strings.TrimSpace(service.mattermostPublicBaseURL()), "/")
	normalizedHomepage := strings.TrimRight(strings.TrimSpace(homepage), "/")
	if normalizedHomepage == "" || normalizedHomepage == defaultHomepage {
		return basePath
	}
	filename := "mattermost-oauth-" + safeMattermostOAuthClientName(normalizedHomepage) + ".json"
	return filepath.Join(filepath.Dir(basePath), filename)
}

func mattermostOAuthHomepageFromRedirectURI(redirectURI string) string {
	parsedURL, errorValue := url.Parse(redirectURI)
	if errorValue != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return ""
	}
	return parsedURL.Scheme + "://" + parsedURL.Host
}

func safeMattermostOAuthClientName(value string) string {
	parsedURL, errorValue := url.Parse(value)
	name := value
	if errorValue == nil && parsedURL.Hostname() != "" {
		name = parsedURL.Hostname()
	}
	name = strings.ToLower(strings.TrimSpace(name))
	builder := strings.Builder{}
	for _, character := range name {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			builder.WriteRune(character)
			continue
		}
		builder.WriteRune('-')
	}
	return strings.Trim(builder.String(), "-")
}

func respondMattermostAuthError(responseWriter http.ResponseWriter, status int, message string) {
	responseWriter.Header().Set("Content-Type", "text/html; charset=utf-8")
	responseWriter.WriteHeader(status)
	_, _ = responseWriter.Write([]byte("<!doctype html><meta charset=\"utf-8\"><title>InternKim Login</title><body style=\"font-family:system-ui,sans-serif;margin:40px\"><h1>로그인할 수 없습니다</h1><p>" + htmlEscape(message) + "</p></body>"))
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}
