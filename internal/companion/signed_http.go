package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const CompanionPairingExpiredMessage = "Pairing expired. Connect again from Admin."

type DeviceClient struct {
	HTTPClient *http.Client
	State      State
	PrivateKey string
}

func (client DeviceClient) SignedJSONRequest(method string, endpoint string, requestBody any, responseBody any) error {
	return client.SignedJSONRequestWithContext(context.Background(), method, endpoint, requestBody, responseBody)
}

func (client DeviceClient) SignedJSONRequestWithContext(ctx context.Context, method string, endpoint string, requestBody any, responseBody any) error {
	var document []byte
	var errorValue error
	if requestBody != nil {
		document, errorValue = json.Marshal(requestBody)
		if errorValue != nil {
			return errorValue
		}
	}
	request, errorValue := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	if requestBody != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for key, value := range CompanionHeaders(client.State) {
		request.Header.Set(key, value)
	}
	if errorValue := SignRequest(request, document, client.PrivateKey); errorValue != nil {
		return errorValue
	}
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	return DecodeJSONResponse(endpoint, response, responseBody)
}

func (client DeviceClient) PostSignedJSON(endpoint string, requestBody any, responseBody any) error {
	return client.SignedJSONRequest(http.MethodPost, endpoint, requestBody, responseBody)
}

func (client DeviceClient) PutSignedBytes(ctx context.Context, endpoint string, requestBody []byte) error {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(requestBody))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	for key, value := range CompanionHeaders(client.State) {
		request.Header.Set(key, value)
	}
	if errorValue := SignRequest(request, requestBody, client.PrivateKey); errorValue != nil {
		return errorValue
	}
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		return errors.New(string(body))
	}
	return nil
}

func (client DeviceClient) httpClient() *http.Client {
	if client.HTTPClient == nil {
		return http.DefaultClient
	}
	return client.HTTPClient
}

func CompanionHeaders(state State) map[string]string {
	return map[string]string{
		"X-INTERNKIM-COMPANION-ID":    state.CompanionID,
		"X-INTERNKIM-COMPANION-TOKEN": state.Token,
	}
}

func DecodeJSONResponse(endpoint string, response *http.Response, responseBody any) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode >= http.StatusBadRequest {
		if response.StatusCode == http.StatusForbidden && IsCompanionAuthRequiredBody(body) {
			return errors.New(CompanionPairingExpiredMessage)
		}
		return fmt.Errorf("%s returned %d: %s", endpoint, response.StatusCode, sanitizedHTTPBody(body))
	}
	if responseBody == nil || len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	trimmedBody := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmedBody, "<") {
		return fmt.Errorf("%s returned HTML instead of JSON; the companion route is probably still behind Cloudflare Access or serving the wrong path", endpoint)
	}
	if errorValue := json.Unmarshal(body, responseBody); errorValue != nil {
		return fmt.Errorf("%s returned invalid JSON: %w", endpoint, errorValue)
	}
	return nil
}

func IsCompanionAuthRequiredError(errorValue error) bool {
	return errorValue != nil && strings.Contains(errorValue.Error(), CompanionPairingExpiredMessage)
}

func IsCompanionAuthRequiredBody(body []byte) bool {
	return strings.Contains(strings.TrimSpace(string(body)), "companion auth required")
}

func sanitizedHTTPBody(body []byte) string {
	trimmedBody := strings.TrimSpace(string(body))
	if trimmedBody == "" {
		return "empty response body"
	}
	if strings.HasPrefix(trimmedBody, "<") {
		return "HTML response"
	}
	if len(trimmedBody) > 512 {
		return trimmedBody[:512]
	}
	return trimmedBody
}
