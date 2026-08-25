package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type Team struct {
	TeamID       string `json:"teamID"`
	Name         string `json:"name"`
	ParentTeamID string `json:"parentTeamID"`
}

type OfferedTeam struct {
	TeamID     string `json:"teamID"`
	Name       string `json:"name"`
	ParentName string `json:"parentName"`
}

func (client *Client) Teams(ctx context.Context) ([]Team, error) {
	var answer struct {
		Teams []Team `json:"teams"`
	}
	if errorValue := client.callTeams(ctx, http.MethodGet, nil, &answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Teams, nil
}

func (client *Client) SettleTeams(ctx context.Context, teams []OfferedTeam) ([]Team, error) {
	body, errorValue := json.Marshal(map[string]any{"teams": teams})
	if errorValue != nil {
		return nil, errorValue
	}
	var answer struct {
		Teams []Team `json:"teams"`
	}
	if errorValue := client.callTeams(ctx, http.MethodPut, body, &answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Teams, nil
}

func (client *Client) callTeams(ctx context.Context, method string, body []byte, answer any) error {
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/team"
	request, errorValue := http.NewRequestWithContext(ctx, method, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the central plane refused the teams: %s", response.Status)
	}
	return json.NewDecoder(response.Body).Decode(answer)
}
