package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OrganizationProfile struct {
	Email           string `json:"email"`
	JobTitle        string `json:"jobTitle"`
	PhoneNumber     string `json:"phoneNumber"`
	HireDate        string `json:"hireDate"`
	SupervisorEmail string `json:"supervisorEmail"`
	TeamName        string `json:"teamName"`
}

func (client *Client) WriteOrganizationProfiles(ctx context.Context, profiles []OrganizationProfile) ([]string, error) {
	if len(profiles) == 0 {
		return nil, nil
	}
	if client == nil || !client.settings.Configured() {
		return nil, fmt.Errorf("central plane is not configured")
	}
	body, errorValue := json.Marshal(map[string]any{"profiles": profiles})
	if errorValue != nil {
		return nil, errorValue
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPatch, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("the central plane refused the organization profiles: %s", response.Status)
	}

	var answer struct {
		Written []string `json:"written"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Written, nil
}
