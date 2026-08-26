package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Member is one entry of the company's account directory, which is the only place a
// person is added or removed. A host reads these to answer with and never authors them.
// A member carries what the company knows about who this is: the identifier it
// issued, the address, what they are called, and the messenger accounts that are
// theirs. Anything that has to turn a name into a person reads it from here, so
// that a name means one person rather than one per list that kept its own copy.
type Member struct {
	MemberID        string            `json:"memberID"`
	Email           string            `json:"email"`
	Name            string            `json:"name"`
	Messenger       map[string]string `json:"messenger"`
	Role            string            `json:"role"`
	Circles         []string          `json:"circles"`
	Status          string            `json:"status"`
	JobTitle        string            `json:"jobTitle"`
	PhoneNumber     string            `json:"phoneNumber"`
	HireDate        string            `json:"hireDate"`
	TeamID          string            `json:"teamID"`
	TeamName        string            `json:"teamName"`
	SupervisorEmail string            `json:"supervisorEmail"`
}

func (member Member) IsActive() bool {
	return strings.EqualFold(strings.TrimSpace(member.Status), "active")
}

// Members asks the directory who works here. Asking address by address cannot
// find somebody nobody has mentioned yet, which is every person invited since
// the caller last looked.
func (client *Client) Members(ctx context.Context) ([]Member, error) {
	if client == nil || !client.settings.Configured() {
		return nil, fmt.Errorf("central plane is not configured")
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
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
		return nil, fmt.Errorf("central plane refused the directory: %s", response.Status)
	}

	var answer struct {
		Members []Member `json:"members"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return nil, errorValue
	}
	return answer.Members, nil
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

func (client *Client) EnsureMember(ctx context.Context, email string, name string) (Member, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return Member{}, fmt.Errorf("a person needs an email address")
	}
	if client == nil || !client.settings.Configured() {
		return Member{}, fmt.Errorf("central plane is not configured")
	}
	body, errorValue := json.Marshal(map[string]string{"email": normalizedEmail, "name": strings.TrimSpace(name)})
	if errorValue != nil {
		return Member{}, errorValue
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return Member{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return Member{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return Member{}, fmt.Errorf("the central plane refused to seat %s: %s", normalizedEmail, response.Status)
	}

	var answer struct {
		Member *Member `json:"member"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return Member{}, errorValue
	}
	if answer.Member == nil || strings.TrimSpace(answer.Member.MemberID) == "" {
		return Member{}, fmt.Errorf("the central plane seated %s without a member id", normalizedEmail)
	}
	return *answer.Member, nil
}

// KeepMessengerCredential writes the credential a person signs into the
// messenger with. The machine derives it from a seed; the record has to hold it,
// or the web messenger - which never sees a seed - is handed whatever credential
// that person had before.
func (client *Client) KeepMessengerCredential(ctx context.Context, memberID string, kind string, externalID string, secret string) error {
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	body, errorValue := json.Marshal(map[string]string{
		"memberID":   memberID,
		"kind":       kind,
		"externalID": externalID,
		"secret":     secret,
	})
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/messenger-credential"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the central plane refused the %s credential for %s: %s", kind, memberID, response.Status)
	}
	return nil
}
