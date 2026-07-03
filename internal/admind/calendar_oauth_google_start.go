package admind

import (
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

type googleOAuthStateRecord struct {
	CreatedAt   time.Time
	RedirectURI string
	ReturnURL   string
	Popup       bool
}

type googleOAuthStartResponse struct {
	Provider         string `json:"provider"`
	Status           string `json:"status"`
	AuthorizationURL string `json:"authorizationURL"`
	RedirectURI      string `json:"redirectURI"`
	ReturnURL        string `json:"returnURL"`
}

type googleOAuthStartError struct {
	Stage string
	Cause error
}

func (errorValue googleOAuthStartError) Error() string {
	return errorValue.Stage + ": " + errorValue.Cause.Error()
}

func (errorValue googleOAuthStartError) Unwrap() error {
	return errorValue.Cause
}

func (service *Service) handleGoogleOAuthStart(writer http.ResponseWriter, request *http.Request) {
	if !service.canManageGoogleOAuth(request) {
		http.Error(writer, "admin access required", http.StatusForbidden)
		return
	}
	service.cleanupExpiredGoogleOAuthStates(time.Now())
	redirectURI := googleOAuthRedirectURIFromRequest(request)
	returnURL := googleOAuthReturnURLFromRequest(request)
	shouldUsePopup := shouldUseGoogleOAuthPopup(request)
	response, errorValue := service.createGoogleOAuthStartResponse(redirectURI, returnURL, shouldSwitchGoogleOAuthAccount(request), shouldUsePopup)
	if errorValue != nil {
		log.Printf("google oauth start: %v", errorValue)
		if shouldUsePopup {
			respondGoogleOAuthPopupResult(writer, request, returnURL, googleOAuthReturnStatusFailed)
			return
		}
		redirectGoogleOAuthResult(writer, request, returnURL, googleOAuthReturnStatusFailed)
		return
	}
	http.Redirect(writer, request, response.AuthorizationURL, http.StatusFound)
}

func (service *Service) createGoogleOAuthStartResponse(redirectURI string, returnURL string, shouldSwitchAccount bool, shouldUsePopup bool) (googleOAuthStartResponse, error) {
	configuration, errorValue := service.buildGoogleOAuthConfig(redirectURI)
	if errorValue != nil {
		return googleOAuthStartResponse{}, googleOAuthStartError{Stage: "client_configuration", Cause: errorValue}
	}
	state, errorValue := generateRandomURLToken(32)
	if errorValue != nil {
		return googleOAuthStartResponse{}, googleOAuthStartError{Stage: "state_generation", Cause: errorValue}
	}
	service.googleOAuthStates.Store(state, &googleOAuthStateRecord{
		CreatedAt:   time.Now().UTC(),
		RedirectURI: redirectURI,
		ReturnURL:   returnURL,
		Popup:       shouldUsePopup,
	})
	authorizationURL := configuration.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", googleOAuthPromptValue(shouldSwitchAccount)),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)
	return googleOAuthStartResponse{
		Provider:         "google",
		Status:           "authorization_required",
		AuthorizationURL: authorizationURL,
		RedirectURI:      redirectURI,
		ReturnURL:        returnURL,
	}, nil
}

func shouldSwitchGoogleOAuthAccount(request *http.Request) bool {
	value := strings.TrimSpace(strings.ToLower(request.URL.Query().Get(googleOAuthSwitchQuery)))
	return value == "true" || value == "1"
}

func shouldUseGoogleOAuthPopup(request *http.Request) bool {
	value := strings.TrimSpace(strings.ToLower(request.URL.Query().Get(googleOAuthPopupQuery)))
	return value == "true" || value == "1"
}

func googleOAuthPromptValue(shouldSwitchAccount bool) string {
	if shouldSwitchAccount {
		return "select_account consent"
	}
	return "consent"
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
