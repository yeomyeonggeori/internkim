package admind

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const (
	googleAuthURL             = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL            = "https://oauth2.googleapis.com/token"
	googleUserinfoURL         = "https://www.googleapis.com/oauth2/v2/userinfo"
	googleCalendarScope       = "https://www.googleapis.com/auth/calendar"
	googleOpenIDScope         = "openid"
	googleUserinfoEmailScope  = "https://www.googleapis.com/auth/userinfo.email"
	googleOAuthStartPath      = "/calendar/oauth/google/start"
	googleOAuthCallbackPath   = "/calendar/oauth/google/callback"
	googleOAuthClientFileName = "client.json"
	googleOAuthAccountPrefix  = "google-"
	googleOAuthStateTTL       = 10 * time.Minute

	googleAuthURLOverrideEnv     = "INTERNKIM_GOOGLE_AUTH_URL"
	googleTokenURLOverrideEnv    = "INTERNKIM_GOOGLE_TOKEN_URL"
	googleUserinfoURLOverrideEnv = "INTERNKIM_GOOGLE_USERINFO_URL"
)

type googleOAuthClientFile struct {
	Installed *googleOAuthClientCredentials `json:"installed,omitempty"`
	Web       *googleOAuthClientCredentials `json:"web,omitempty"`
}

type googleOAuthClientCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type googleOAuthStateRecord struct {
	CreatedAt   time.Time
	RedirectURI string
}

func googleAuthorizeEndpoint() string {
	if override := strings.TrimSpace(os.Getenv(googleAuthURLOverrideEnv)); override != "" {
		return override
	}
	return googleAuthURL
}

func googleTokenEndpoint() string {
	if override := strings.TrimSpace(os.Getenv(googleTokenURLOverrideEnv)); override != "" {
		return override
	}
	return googleTokenURL
}

func googleUserinfoEndpoint() string {
	if override := strings.TrimSpace(os.Getenv(googleUserinfoURLOverrideEnv)); override != "" {
		return override
	}
	return googleUserinfoURL
}

func (service *Service) googleOAuthClientFilePath() string {
	return filepath.Join(service.calendarSecretsDirectory(), googleOAuthClientFileName)
}

func (service *Service) loadGoogleOAuthClientSecret() (string, string, error) {
	path := service.googleOAuthClientFilePath()
	payload, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return "", "", fmt.Errorf("read google oauth client file %s: %w", path, errorValue)
	}
	var parsed googleOAuthClientFile
	if errorValue := json.Unmarshal(payload, &parsed); errorValue != nil {
		return "", "", fmt.Errorf("parse google oauth client file: %w", errorValue)
	}
	if parsed.Installed != nil && strings.TrimSpace(parsed.Installed.ClientID) != "" {
		return parsed.Installed.ClientID, parsed.Installed.ClientSecret, nil
	}
	if parsed.Web != nil && strings.TrimSpace(parsed.Web.ClientID) != "" {
		return parsed.Web.ClientID, parsed.Web.ClientSecret, nil
	}
	return "", "", errors.New("google oauth client file missing client_id")
}

func (service *Service) buildGoogleOAuthConfig(redirectURI string) (*oauth2.Config, error) {
	clientID, clientSecret, errorValue := service.loadGoogleOAuthClientSecret()
	if errorValue != nil {
		return nil, errorValue
	}
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  googleAuthorizeEndpoint(),
			TokenURL: googleTokenEndpoint(),
		},
		RedirectURL: redirectURI,
		Scopes:      []string{googleCalendarScope, googleOpenIDScope, googleUserinfoEmailScope},
	}, nil
}

func (service *Service) handleGoogleOAuthStart(writer http.ResponseWriter, request *http.Request) {
	service.cleanupExpiredGoogleOAuthStates(time.Now())
	redirectURI := googleOAuthRedirectURIFromRequest(request)
	configuration, errorValue := service.buildGoogleOAuthConfig(redirectURI)
	if errorValue != nil {
		log.Printf("google oauth start: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, http.StatusInternalServerError, "OAuth 설정을 불러올 수 없습니다. client.json 을 확인해주세요.")
		return
	}
	state, errorValue := generateRandomURLToken(32)
	if errorValue != nil {
		log.Printf("google oauth state generation: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, http.StatusInternalServerError, "내부 오류로 state 생성에 실패했습니다.")
		return
	}
	service.googleOAuthStates.Store(state, &googleOAuthStateRecord{
		CreatedAt:   time.Now().UTC(),
		RedirectURI: redirectURI,
	})
	authorizeURL := configuration.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	http.Redirect(writer, request, authorizeURL, http.StatusFound)
}

func (service *Service) handleGoogleOAuthCallback(writer http.ResponseWriter, request *http.Request) {
	service.cleanupExpiredGoogleOAuthStates(time.Now())
	state, code, ok := readGoogleOAuthCallbackParams(writer, request)
	if !ok {
		return
	}
	record, ok := service.consumeGoogleOAuthState(writer, state)
	if !ok {
		return
	}
	account, ok := service.completeGoogleOAuthExchange(writer, request.Context(), record, code)
	if !ok {
		return
	}
	respondGoogleOAuthSuccessHTML(writer, account.AccountEmail)
}

func readGoogleOAuthCallbackParams(writer http.ResponseWriter, request *http.Request) (string, string, bool) {
	query := request.URL.Query()
	if callbackError := strings.TrimSpace(query.Get("error")); callbackError != "" {
		respondGoogleOAuthErrorHTML(writer, http.StatusBadRequest,
			fmt.Sprintf("Google이 오류를 반환했습니다: %s", callbackError))
		return "", "", false
	}
	state := strings.TrimSpace(query.Get("state"))
	code := strings.TrimSpace(query.Get("code"))
	if state == "" || code == "" {
		respondGoogleOAuthErrorHTML(writer, http.StatusBadRequest, "state 또는 code 파라미터가 누락되었습니다.")
		return "", "", false
	}
	return state, code, true
}

func (service *Service) consumeGoogleOAuthState(writer http.ResponseWriter, state string) (*googleOAuthStateRecord, bool) {
	recordRaw, found := service.googleOAuthStates.LoadAndDelete(state)
	if !found {
		respondGoogleOAuthErrorHTML(writer, http.StatusBadRequest, "잘못되었거나 이미 사용된 state 입니다. 처음부터 다시 시도해주세요.")
		return nil, false
	}
	record, recordOK := recordRaw.(*googleOAuthStateRecord)
	if !recordOK {
		respondGoogleOAuthErrorHTML(writer, http.StatusInternalServerError, "state 레코드 타입 오류입니다.")
		return nil, false
	}
	if time.Since(record.CreatedAt) > googleOAuthStateTTL {
		respondGoogleOAuthErrorHTML(writer, http.StatusBadRequest, "state 가 만료되었습니다. 다시 시작해주세요.")
		return nil, false
	}
	return record, true
}

func (service *Service) completeGoogleOAuthExchange(writer http.ResponseWriter, ctx context.Context, record *googleOAuthStateRecord, code string) (remoteCalendarAccount, bool) {
	configuration, errorValue := service.buildGoogleOAuthConfig(record.RedirectURI)
	if errorValue != nil {
		log.Printf("google oauth callback config: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, http.StatusInternalServerError, "OAuth 설정 로드에 실패했습니다.")
		return remoteCalendarAccount{}, false
	}
	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, service.googleOAuthHTTPClient())
	token, errorValue := configuration.Exchange(exchangeCtx, code)
	if errorValue != nil {
		log.Printf("google oauth exchange: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, http.StatusBadGateway, "Google 토큰 교환에 실패했습니다.")
		return remoteCalendarAccount{}, false
	}
	email, errorValue := service.fetchGoogleUserEmail(ctx, token)
	if errorValue != nil {
		log.Printf("google oauth userinfo: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, http.StatusBadGateway, "Google 사용자 정보 조회에 실패했습니다.")
		return remoteCalendarAccount{}, false
	}
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, email)
	if errorValue != nil {
		log.Printf("google oauth save: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, http.StatusInternalServerError, "토큰 저장에 실패했습니다.")
		return remoteCalendarAccount{}, false
	}
	return account, true
}

func (service *Service) cleanupExpiredGoogleOAuthStates(now time.Time) {
	service.googleOAuthStates.Range(func(key, value any) bool {
		record, ok := value.(*googleOAuthStateRecord)
		if !ok || now.Sub(record.CreatedAt) > googleOAuthStateTTL {
			service.googleOAuthStates.Delete(key)
		}
		return true
	})
}

func (service *Service) googleOAuthHTTPClient() *http.Client {
	if service.HTTPClient != nil {
		return service.HTTPClient
	}
	return http.DefaultClient
}

func (service *Service) fetchGoogleUserEmail(ctx context.Context, token *oauth2.Token) (string, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, googleUserinfoEndpoint(), nil)
	if errorValue != nil {
		return "", errorValue
	}
	token.SetAuthHeader(request)
	response, errorValue := service.googleOAuthHTTPClient().Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("google userinfo status %d", response.StatusCode)
	}
	var parsed struct {
		Email string `json:"email"`
	}
	if errorValue := json.Unmarshal(body, &parsed); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(parsed.Email) == "" {
		return "", errors.New("google userinfo response missing email")
	}
	return parsed.Email, nil
}

func (service *Service) saveGoogleOAuthTokenAndAccount(ctx context.Context, token *oauth2.Token, email string) (remoteCalendarAccount, error) {
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	accountID := googleOAuthAccountPrefix + sanitizeCalendarSecretComponent(email)
	tokenPath := service.calendarTokenFilePath(accountID)
	payload := oauthTokenPayloadFromOAuth2Token(token)
	if errorValue := writeCalendarTokenFile(tokenPath, key, payload); errorValue != nil {
		return remoteCalendarAccount{}, errorValue
	}
	existing, _, _ := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	account := remoteCalendarAccount{
		ID:                  accountID,
		Provider:            remoteCalendarProviderGoogle,
		AccountEmail:        email,
		PrincipalURL:        existing.PrincipalURL,
		HomeSetURL:          existing.HomeSetURL,
		DefaultCalendarURL:  existing.DefaultCalendarURL,
		DefaultCalendarCTag: existing.DefaultCalendarCTag,
		TokenFilePath:       tokenPath,
	}
	return service.upsertRemoteCalendarAccount(ctx, account)
}

func (service *Service) loadGoogleOAuthTokenForAccount(account remoteCalendarAccount) (*oauth2.Token, error) {
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return nil, errorValue
	}
	payload, errorValue := readCalendarTokenFile(account.TokenFilePath, key)
	if errorValue != nil {
		return nil, errorValue
	}
	return oauth2TokenFromPayload(payload), nil
}

func (service *Service) googleOAuthTokenSource(ctx context.Context, account remoteCalendarAccount, redirectURI string) (oauth2.TokenSource, error) {
	initialToken, errorValue := service.loadGoogleOAuthTokenForAccount(account)
	if errorValue != nil {
		return nil, errorValue
	}
	configuration, errorValue := service.buildGoogleOAuthConfig(redirectURI)
	if errorValue != nil {
		return nil, errorValue
	}
	clientCtx := context.WithValue(ctx, oauth2.HTTPClient, service.googleOAuthHTTPClient())
	baseSource := configuration.TokenSource(clientCtx, initialToken)
	return &persistingGoogleOAuthTokenSource{
		service: service,
		account: account,
		inner:   baseSource,
		latest:  initialToken,
	}, nil
}

type persistingGoogleOAuthTokenSource struct {
	service *Service
	account remoteCalendarAccount
	inner   oauth2.TokenSource
	latest  *oauth2.Token
}

func (source *persistingGoogleOAuthTokenSource) Token() (*oauth2.Token, error) {
	token, errorValue := source.inner.Token()
	if errorValue != nil {
		return nil, errorValue
	}
	if source.latest != nil && token.AccessToken == source.latest.AccessToken && token.Expiry.Equal(source.latest.Expiry) {
		return token, nil
	}
	source.latest = token
	key, errorValue := source.service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		return token, errorValue
	}
	payload := oauthTokenPayloadFromOAuth2Token(token)
	if errorValue := writeCalendarTokenFile(source.account.TokenFilePath, key, payload); errorValue != nil {
		return token, errorValue
	}
	return token, nil
}

func oauthTokenPayloadFromOAuth2Token(token *oauth2.Token) oauthTokenPayload {
	expiresAt := ""
	if !token.Expiry.IsZero() {
		expiresAt = token.Expiry.UTC().Format(time.RFC3339Nano)
	}
	return oauthTokenPayload{
		AccessToken:      token.AccessToken,
		RefreshToken:     token.RefreshToken,
		TokenType:        token.TokenType,
		ExpiresAtRFC3339: expiresAt,
	}
}

func oauth2TokenFromPayload(payload oauthTokenPayload) *oauth2.Token {
	token := &oauth2.Token{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		TokenType:    payload.TokenType,
	}
	if payload.ExpiresAtRFC3339 != "" {
		if parsed, errorValue := time.Parse(time.RFC3339Nano, payload.ExpiresAtRFC3339); errorValue == nil {
			token.Expiry = parsed
		}
	}
	return token
}

func googleOAuthRedirectURIFromRequest(request *http.Request) string {
	scheme := "http"
	if request.TLS != nil {
		scheme = "https"
	}
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	return scheme + "://" + request.Host + googleOAuthCallbackPath
}

func generateRandomURLToken(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, errorValue := rand.Read(buffer); errorValue != nil {
		return "", errorValue
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func respondGoogleOAuthSuccessHTML(writer http.ResponseWriter, email string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(writer, `<!doctype html>
<html lang="ko">
<head><meta charset="utf-8"><title>Google 캘린더 연결 완료</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}h1{color:#2563eb}</style>
</head>
<body>
<h1>Google 캘린더 연결 완료</h1>
<p><strong>%s</strong> 계정으로 연결되었습니다. 이 탭은 닫아도 됩니다.</p>
</body>
</html>`, html.EscapeString(email))
}

func respondGoogleOAuthErrorHTML(writer http.ResponseWriter, status int, message string) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	fmt.Fprintf(writer, `<!doctype html>
<html lang="ko">
<head><meta charset="utf-8"><title>Google 캘린더 연결 실패</title>
<style>body{font-family:system-ui,sans-serif;margin:4rem auto;max-width:32rem;padding:0 1rem;line-height:1.6}h1{color:#dc2626}</style>
</head>
<body>
<h1>Google 캘린더 연결 실패</h1>
<p>%s</p>
</body>
</html>`, html.EscapeString(message))
}
