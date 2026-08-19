package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Member is one row of the company's account directory, which is the only place a
// person is added or removed. A host holds these to answer with; it never authors them.
type Member struct {
	MemberID string `json:"memberID"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func (member Member) IsActive() bool {
	return strings.EqualFold(strings.TrimSpace(member.Status), "active")
}

// Members reads the company's account directory. It returns an error rather than an
// empty roster when the central plane cannot be reached, because a caller that cannot
// tell those apart will project "nobody works here" onto the host.
func (client *Client) Members(ctx context.Context) ([]Member, error) {
	if client == nil || !client.settings.Configured() {
		return nil, fmt.Errorf("central plane is not configured")
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/agent/members", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("central plane refused the member list: %s", response.Status)
	}

	var answer struct {
		Members []Member `json:"members"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Members, nil
}
