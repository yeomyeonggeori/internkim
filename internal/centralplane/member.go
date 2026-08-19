package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Member is one entry of the company's account directory, which is the only place a
// person is added or removed. A host reads these to answer with and never authors them.
// A member carries no display name here on purpose: what a person is called is what
// their messenger account presents, which the caller already holds, and reading it from
// the directory would tie this lookup to a column the account record does not need.
type Member struct {
	MemberID string `json:"memberID"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

func (member Member) IsActive() bool {
	return strings.EqualFold(strings.TrimSpace(member.Status), "active")
}

// MemberByEmail asks the directory about one address. A directory that cannot be reached
// returns an error and never an absent member, because a caller that cannot tell those
// apart will refuse a colleague over a timeout.
func (client *Client) MemberByEmail(ctx context.Context, email string) (Member, bool, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return Member{}, false, nil
	}
	if client == nil || !client.settings.Configured() {
		return Member{}, false, fmt.Errorf("central plane is not configured")
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member?email=" + url.QueryEscape(normalizedEmail)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return Member{}, false, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return Member{}, false, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return Member{}, false, fmt.Errorf("central plane refused the member lookup: %s", response.Status)
	}

	var answer struct {
		Member *Member `json:"member"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return Member{}, false, errorValue
	}
	if answer.Member == nil {
		return Member{}, false, nil
	}
	return *answer.Member, true, nil
}
