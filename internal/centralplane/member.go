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
	Note            string            `json:"note"`
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
	return strings.EqualFold(strings.TrimSpace(member.Status), MemberStatusActive)
}

// Somebody invited yesterday and signing in tomorrow is neither active nor
// gone, and what is prepared for them ahead of that sign-in reads this rather
// than IsActive: a key derived the day they are invited is waiting the moment
// they arrive. Mirrors hasLeftTheCompany in web/src/lib/server/control-plane.ts.
func (member Member) HasLeftTheCompany() bool {
	status := strings.ToLower(strings.TrimSpace(member.Status))
	return status == MemberStatusDeparted || status == MemberStatusWithdrawn
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

// Seating somebody goes through person_invite, which is the plane's own invite
// path: it creates their account and their sign-in rather than writing a member
// row nobody can use. Somebody already seated keeps the seat they have.
func (client *Client) EnsureMember(ctx context.Context, email string, name string) (Member, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return Member{}, fmt.Errorf("a person needs an email address")
	}
	if client == nil || !client.settings.Configured() {
		return Member{}, fmt.Errorf("central plane is not configured")
	}
	administratorEmail, errorValue := client.claimedAdministratorEmail()
	if errorValue != nil {
		return Member{}, errorValue
	}

	seated, isSeated, errorValue := client.MemberByEmail(ctx, normalizedEmail)
	if errorValue != nil {
		return Member{}, errorValue
	}
	if isSeated {
		return client.renamedMember(ctx, administratorEmail, seated, name)
	}
	return client.invitedMember(ctx, administratorEmail, normalizedEmail, name)
}

func (client *Client) renamedMember(
	ctx context.Context,
	administratorEmail string,
	seated Member,
	name string,
) (Member, error) {
	wanted := strings.TrimSpace(name)
	if wanted == "" || wanted == strings.TrimSpace(seated.Name) {
		return seated, nil
	}
	var written struct {
		Name string `json:"name"`
	}
	errorValue := client.runRecordTool(ctx, administratorEmail, "person_update",
		map[string]string{"personHint": seated.MemberID, "name": wanted}, &written)
	if errorValue != nil {
		return Member{}, errorValue
	}
	seated.Name = written.Name
	return seated, nil
}

func (client *Client) invitedMember(
	ctx context.Context,
	administratorEmail string,
	email string,
	name string,
) (Member, error) {
	invitation := map[string]string{"email": email, "name": strings.TrimSpace(name)}
	if invitation["name"] == "" {
		invitation["name"] = email
	}
	var invited struct {
		PersonID         string `json:"personID"`
		Email            string `json:"email"`
		Name             string `json:"name"`
		EmploymentStatus string `json:"employmentStatus"`
	}
	if errorValue := client.runRecordTool(ctx, administratorEmail, "person_invite", invitation, &invited); errorValue != nil {
		return Member{}, errorValue
	}
	if strings.TrimSpace(invited.PersonID) == "" {
		return Member{}, fmt.Errorf("the central plane seated %s without a member id", email)
	}
	return Member{
		MemberID: invited.PersonID,
		Email:    invited.Email,
		Name:     invited.Name,
		Role:     MemberRoleMember,
		Status:   invited.EmploymentStatus,
	}, nil
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

func (client *Client) SaveMember(ctx context.Context, write MemberWrite) (Member, error) {
	write.Email = strings.ToLower(strings.TrimSpace(write.Email))
	if write.Email == "" {
		return Member{}, fmt.Errorf("a person needs an email address")
	}
	if client == nil || !client.settings.Configured() {
		return Member{}, fmt.Errorf("central plane is not configured")
	}
	body, errorValue := json.Marshal(write)
	if errorValue != nil {
		return Member{}, errorValue
	}
	requestURL := strings.TrimSuffix(client.settings.AppURL, "/") + "/api/agent/member"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if errorValue != nil {
		return Member{}, errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.HostCredential())
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return Member{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return Member{}, fmt.Errorf("the central plane refused to save %s: %s", write.Email, response.Status)
	}

	var answer struct {
		Member *Member `json:"member"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return Member{}, errorValue
	}
	if answer.Member == nil {
		return Member{}, fmt.Errorf("the central plane saved %s without answering who that is", write.Email)
	}
	return *answer.Member, nil
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
