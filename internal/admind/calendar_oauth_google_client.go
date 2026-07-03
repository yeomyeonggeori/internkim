package admind

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
)

type googleOAuthClientFile struct {
	Installed *googleOAuthClientCredentials `json:"installed,omitempty"`
	Web       *googleOAuthClientCredentials `json:"web,omitempty"`
}

type googleOAuthClientCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
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
	credentials, errorValue := parseGoogleOAuthClientDocument(payload)
	if errorValue != nil {
		return "", "", errorValue
	}
	return credentials.ClientID, credentials.ClientSecret, nil
}

func parseGoogleOAuthClientDocument(document []byte) (googleOAuthClientCredentials, error) {
	var parsed googleOAuthClientFile
	if errorValue := json.Unmarshal(document, &parsed); errorValue != nil {
		return googleOAuthClientCredentials{}, fmt.Errorf("parse google oauth client file: %w", errorValue)
	}
	credentials := parsed.Installed
	if credentials == nil || strings.TrimSpace(credentials.ClientID) == "" {
		credentials = parsed.Web
	}
	if credentials == nil || strings.TrimSpace(credentials.ClientID) == "" {
		return googleOAuthClientCredentials{}, errors.New("google oauth client file missing client_id")
	}
	if strings.TrimSpace(credentials.ClientSecret) == "" {
		return googleOAuthClientCredentials{}, errors.New("google oauth client file missing client_secret")
	}
	return googleOAuthClientCredentials{
		ClientID:     strings.TrimSpace(credentials.ClientID),
		ClientSecret: strings.TrimSpace(credentials.ClientSecret),
	}, nil
}

func (service *Service) isGoogleOAuthConfigured() bool {
	_, _, errorValue := service.loadGoogleOAuthClientSecret()
	return errorValue == nil
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
