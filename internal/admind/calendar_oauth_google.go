package admind

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
	if !service.isAuthorized(request) {
		http.Error(writer, "admin access required", http.StatusForbidden)
		return
	}
	service.cleanupExpiredGoogleOAuthStates(time.Now())
	redirectURI := googleOAuthRedirectURIFromRequest(request)
	configuration, errorValue := service.buildGoogleOAuthConfig(redirectURI)
	if errorValue != nil {
		log.Printf("google oauth start: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, request, http.StatusInternalServerError, googleOAuthResponseTextForRequest(request).ClientConfigurationError)
		return
	}
	state, errorValue := generateRandomURLToken(32)
	if errorValue != nil {
		log.Printf("google oauth state generation: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, request, http.StatusInternalServerError, googleOAuthResponseTextForRequest(request).StateGenerationError)
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
	record, ok := service.consumeGoogleOAuthState(writer, request, state)
	if !ok {
		return
	}
	account, ok := service.completeGoogleOAuthExchange(writer, request, record, code)
	if !ok {
		return
	}
	respondGoogleOAuthSuccessHTML(writer, request, account.AccountEmail)
}

func readGoogleOAuthCallbackParams(writer http.ResponseWriter, request *http.Request) (string, string, bool) {
	query := request.URL.Query()
	text := googleOAuthResponseTextForRequest(request)
	if callbackError := strings.TrimSpace(query.Get("error")); callbackError != "" {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest,
			fmt.Sprintf(text.GoogleReturnedErrorTemplate, callbackError))
		return "", "", false
	}
	state := strings.TrimSpace(query.Get("state"))
	code := strings.TrimSpace(query.Get("code"))
	if state == "" || code == "" {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest, text.MissingCallbackParameters)
		return "", "", false
	}
	return state, code, true
}

func (service *Service) consumeGoogleOAuthState(writer http.ResponseWriter, request *http.Request, state string) (*googleOAuthStateRecord, bool) {
	text := googleOAuthResponseTextForRequest(request)
	recordRaw, found := service.googleOAuthStates.LoadAndDelete(state)
	if !found {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest, text.InvalidState)
		return nil, false
	}
	record, recordOK := recordRaw.(*googleOAuthStateRecord)
	if !recordOK {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusInternalServerError, text.StateRecordTypeError)
		return nil, false
	}
	if time.Since(record.CreatedAt) > googleOAuthStateTTL {
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadRequest, text.ExpiredState)
		return nil, false
	}
	return record, true
}

func (service *Service) completeGoogleOAuthExchange(writer http.ResponseWriter, request *http.Request, record *googleOAuthStateRecord, code string) (remoteCalendarAccount, bool) {
	ctx := request.Context()
	text := googleOAuthResponseTextForRequest(request)
	configuration, errorValue := service.buildGoogleOAuthConfig(record.RedirectURI)
	if errorValue != nil {
		log.Printf("google oauth callback config: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, request, http.StatusInternalServerError, text.OAuthConfigurationError)
		return remoteCalendarAccount{}, false
	}
	exchangeCtx := context.WithValue(ctx, oauth2.HTTPClient, service.googleOAuthHTTPClient())
	token, errorValue := configuration.Exchange(exchangeCtx, code)
	if errorValue != nil {
		log.Printf("google oauth exchange: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadGateway, text.TokenExchangeError)
		return remoteCalendarAccount{}, false
	}
	email, errorValue := service.fetchGoogleUserEmail(ctx, token)
	if errorValue != nil {
		log.Printf("google oauth userinfo: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, request, http.StatusBadGateway, text.UserinfoError)
		return remoteCalendarAccount{}, false
	}
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, email)
	if errorValue != nil {
		log.Printf("google oauth save: %v", errorValue)
		respondGoogleOAuthErrorHTML(writer, request, http.StatusInternalServerError, text.TokenSaveError)
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

func generateRandomURLToken(byteCount int) (string, error) {
	buffer := make([]byte, byteCount)
	if _, errorValue := rand.Read(buffer); errorValue != nil {
		return "", errorValue
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
