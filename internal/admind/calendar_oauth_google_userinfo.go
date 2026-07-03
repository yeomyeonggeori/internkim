package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
)

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
