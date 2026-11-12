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
	MemberID          string            `json:"memberID"`
	Email             string            `json:"email"`
	Name              string            `json:"name"`
	Note              string            `json:"note"`
	Messenger         map[string]string `json:"messenger"`
	Role              string            `json:"role"`
	Circles           []string          `json:"circles"`
	Status            string            `json:"status"`
	IsWorking         bool              `json:"isWorking"`
	CanReadTheCompany bool              `json:"canReadTheCompany"`
	JobTitle          string            `json:"jobTitle"`
	PhoneNumber       string            `json:"phoneNumber"`
	HireDate          string            `json:"hireDate"`
	TeamID            string            `json:"teamID"`
	TeamName          string            `json:"teamName"`
	SupervisorEmail   string            `json:"supervisorEmail"`
}

func (member *Member) UnmarshalJSON(document []byte) error {
	type memberFields Member
	answered := struct {
		*memberFields
		IsWorking         *bool `json:"isWorking"`
		CanReadTheCompany *bool `json:"canReadTheCompany"`
	}{memberFields: (*memberFields)(member)}
	if errorValue := json.Unmarshal(document, &answered); errorValue != nil {
		return errorValue
	}
	if answered.IsWorking == nil || answered.CanReadTheCompany == nil {
		return fmt.Errorf("the central plane named member %q without saying whether they work here or may read the company; it is older than this host and has to be deployed first", member.MemberID)
	}
	member.IsWorking = *answered.IsWorking
	member.CanReadTheCompany = *answered.CanReadTheCompany
	return nil
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
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())

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
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())

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

// A directory write belongs to an administrator, and a sync no person asked
// for has none to name. The device knows which administrator claimed it, and
// that is the identity the record attributes these writes to.
func (client *Client) claimedAdministratorEmail() (string, error) {
	if client == nil || client.settings.ClaimedAdministratorEmail == nil {
		return "", fmt.Errorf("this device names no claimed administrator, so it has nobody to write the directory as")
	}
	email := strings.ToLower(strings.TrimSpace(client.settings.ClaimedAdministratorEmail()))
	if email == "" {
		return "", fmt.Errorf("no administrator has claimed this device, so it has nobody to write the directory as")
	}
	return email, nil
}

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
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())
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

// MemberWrite carries the account concerns of one person: what the company calls
// them, whether they administer it, the note kept about them, and the messenger
// accounts that are theirs. A field left empty keeps what the directory already
// holds, so a caller that knows only the role does not blank the name on its way
// past. Organization attributes (title, team, supervisor, hire date) are not
// account concerns and have no field here.
type MemberWrite struct {
	Email     string            `json:"email"`
	Name      string            `json:"name,omitempty"`
	Role      string            `json:"role,omitempty"`
	Note      string            `json:"note,omitempty"`
	Messenger map[string]string `json:"messenger,omitempty"`
}

func (client *Client) SaveMember(ctx context.Context, write MemberWrite) error {
	write.Email = strings.ToLower(strings.TrimSpace(write.Email))
	if write.Email == "" {
		return fmt.Errorf("a person needs an email address")
	}
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	body, errorValue := json.Marshal(write)
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the central plane refused to save %s: %s", write.Email, response.Status)
	}

	var answer struct {
		Member *struct {
			MemberID string `json:"memberID"`
		} `json:"member"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return errorValue
	}
	if answer.Member == nil || strings.TrimSpace(answer.Member.MemberID) == "" {
		return fmt.Errorf("the central plane saved %s without answering who that is", write.Email)
	}
	return nil
}

// WithdrawMember marks somebody as having left. The row stays, because the work
// they did still names them.
func (client *Client) WithdrawMember(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return fmt.Errorf("a person needs an email address")
	}
	if client == nil || !client.settings.Configured() {
		return fmt.Errorf("central plane is not configured")
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member?email=" + url.QueryEscape(normalizedEmail)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodDelete, requestURL, nil)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("the central plane refused to withdraw %s: %s", normalizedEmail, response.Status)
	}
	return nil
}
