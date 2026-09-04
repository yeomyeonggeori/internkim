package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"time"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type firstAdminPasswordDocument struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type firstAdminBootstrapResult struct {
	Email         string `json:"email"`
	Status        string `json:"status"`
	PolicyVersion string `json:"policyVersion,omitempty"`
	Error         string `json:"error,omitempty"`
}

const firstAdminBootstrapPending = "pending"

const firstAdminBootstrapClaimed = "claimed"

const firstAdminBootstrapIdentityMissing = "identity_missing"

const firstAdminBootstrapRejected = "rejected"

const firstAdminBootstrapFailed = "failed"

const firstAdminPolicyVersion = "blueclaw-admin-claim-v1"

const firstAdminClaimTimeout = 90 * time.Second

func (service *Service) ensureFirstAdminClaim(ctx context.Context, callerEmail string) firstAdminBootstrapResult {
	normalizedEmail := strings.ToLower(strings.TrimSpace(callerEmail))
	if normalizedEmail == "" {
		return firstAdminBootstrapResult{Status: firstAdminBootstrapIdentityMissing}
	}

	claimContext, cancel := context.WithTimeout(context.WithoutCancel(ctx), firstAdminClaimTimeout)
	defer cancel()

	claimedEmail := service.claimedAdminEmail()
	if claimedEmail != "" {
		if strings.EqualFold(claimedEmail, normalizedEmail) {
			return service.ensureClaimedFirstAdminAccount(claimContext, normalizedEmail)
		}
		return firstAdminBootstrapResult{Email: normalizedEmail, Status: firstAdminBootstrapRejected, Error: "first admin is already claimed by another email"}
	}
	if hasAdmin, errorValue := service.hasCurrentAdminUsers(claimContext); errorValue == nil && hasAdmin {
		return firstAdminBootstrapResult{Email: normalizedEmail, Status: firstAdminBootstrapRejected, Error: "first admin is already claimed by another email"}
	}

	result, errorValue := service.claimFirstAdmin(claimContext, normalizedEmail)
	if errorValue != nil {
		log.Printf("first admin bootstrap failed for %s: %v", normalizedEmail, errorValue)
		return service.writeFirstAdminBootstrapResult(firstAdminBootstrapResult{Email: normalizedEmail, Status: firstAdminBootstrapFailed, Error: errorValue.Error()})
	}
	log.Printf("first admin bootstrap claimed by %s", normalizedEmail)
	return service.writeFirstAdminBootstrapResult(result)
}

func (service *Service) ensureClaimedFirstAdminAccount(ctx context.Context, email string) firstAdminBootstrapResult {
	currentResult := service.readFirstAdminBootstrapResult()
	isCurrentAdmin := service.isCurrentAdminEmail(ctx, email)
	if currentResult.PolicyVersion == firstAdminPolicyVersion && isCurrentAdmin {
		currentResult.Email = email
		currentResult.Status = firstAdminBootstrapClaimed
		return currentResult
	}
	if hasAdmin, errorValue := service.hasCurrentAdminUsers(ctx); errorValue == nil && hasAdmin && !isCurrentAdmin {
		currentResult.Email = email
		currentResult.Status = firstAdminBootstrapClaimed
		return currentResult
	}
	result, errorValue := service.ensureFirstAdminAccount(ctx, email)
	if errorValue != nil {
		log.Printf("first admin repair failed for %s: %v", email, errorValue)
		return service.writeFirstAdminBootstrapResult(firstAdminBootstrapResult{Email: email, Status: firstAdminBootstrapFailed, Error: errorValue.Error()})
	}
	if !isCurrentAdmin {
		if errorValue := service.writeClaimedAdminRole(ctx, email); errorValue != nil {
			log.Printf("first admin role repair failed for %s: %v", email, errorValue)
			return service.writeFirstAdminBootstrapResult(firstAdminBootstrapResult{Email: email, Status: firstAdminBootstrapFailed, Error: errorValue.Error()})
		}
	}
	return service.writeFirstAdminBootstrapResult(result)
}

func (service *Service) claimFirstAdmin(ctx context.Context, email string) (firstAdminBootstrapResult, error) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		return firstAdminBootstrapResult{}, fmt.Errorf("device auth is not configured")
	}

	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if errorValue := service.writeUserRole(ctx, fleetID, fleetSecret, email, "admin"); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	for _, record := range records {
		if record.Role != "admin" || strings.EqualFold(record.Email, email) {
			continue
		}
		if errorValue := service.writeUserRole(ctx, fleetID, fleetSecret, record.Email, "member"); errorValue != nil {
			return firstAdminBootstrapResult{}, errorValue
		}
	}
	bootstrapResult, errorValue := service.ensureFirstAdminAccount(ctx, email)
	if errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.ClaimedAdminEmailPath), 0o700); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	if errorValue := os.WriteFile(service.Configuration.ClaimedAdminEmailPath, []byte(email), 0o600); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	_ = os.WriteFile(service.Configuration.AdminEmailPath, []byte(email), 0o644)
	return bootstrapResult, nil
}

// The first admin is a person the company already knows: they founded it on
// the record and signed in there. This claims that person on the device, and
// mints nothing on their behalf.
func (service *Service) ensureFirstAdminAccount(ctx context.Context, email string) (firstAdminBootstrapResult, error) {
	if errorValue := service.claimBlueclawAdminPerson(ctx, email); errorValue != nil {
		return firstAdminBootstrapResult{}, errorValue
	}
	service.triggerUsersSync(ctx)
	return firstAdminBootstrapResult{
		Email:         email,
		Status:        firstAdminBootstrapClaimed,
		PolicyVersion: firstAdminPolicyVersion,
	}, nil
}

func (service *Service) claimBlueclawAdminPerson(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return fmt.Errorf("email required")
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	policyDocument["people"] = claimedAdminPeople(policyDocument["people"], normalizedEmail)
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}

func claimedAdminPeople(value any, email string) []map[string]any {
	existingPeople, _ := value.([]any)
	claimedPeople := make([]map[string]any, 0, len(existingPeople)+1)
	adminPerson := map[string]any{}
	for _, value := range existingPeople {
		person, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if isBlueclawAdminPerson(person) {
			adminPerson = person
			continue
		}
		remainingEmails := blueclawPersonEmailsExcept(person, email)
		if len(remainingEmails) == 0 {
			continue
		}
		person["emails"] = remainingEmails
		claimedPeople = append(claimedPeople, person)
	}
	claimedPeople = append([]map[string]any{claimedAdminPerson(adminPerson, email)}, claimedPeople...)
	return claimedPeople
}

func isBlueclawAdminPerson(person map[string]any) bool {
	personID, _ := person["personID"].(string)
	if personID == blueclawruntime.BlueclawPolicyAdminID {
		return true
	}
	isAdmin, _ := person["isAdmin"].(bool)
	return isAdmin
}

func claimedAdminPerson(person map[string]any, email string) map[string]any {
	if person == nil {
		person = map[string]any{}
	}
	person["personID"] = blueclawruntime.BlueclawPolicyAdminID
	person["displayName"] = "Intern Kim Admin"
	person["emails"] = []string{email}
	person["circles"] = []string{"member", "admin"}
	person["securityLevelName"] = "admin"
	person["securityLevelRank"] = 100
	person["grantedClasses"] = []string{"internal", "executive"}
	person["isAdmin"] = true
	return person
}

func blueclawPersonEmailsExcept(person map[string]any, excludedEmail string) []string {
	values, _ := person["emails"].([]any)
	emails := make([]string, 0, len(values))
	for _, value := range values {
		email, ok := value.(string)
		if !ok {
			continue
		}
		normalizedEmail := strings.ToLower(strings.TrimSpace(email))
		if normalizedEmail == "" || normalizedEmail == excludedEmail {
			continue
		}
		emails = append(emails, normalizedEmail)
	}
	return emails
}

// After an organization change the caller gets the directory back, so the
// screen it just edited redraws from what the company holds rather than from
// what the browser thought it was sending.
func (service *Service) writeFullLocalUsersResponse(responseWriter http.ResponseWriter, request *http.Request) {
	records := service.accountDirectoryUserRecords(request)
	service.writeDirectoryUsersResponse(responseWriter, request, pagesUsersResponse{Records: records})
}

func policyStringList(value any) []string {
	values, _ := value.([]any)
	result := []string{}
	for _, item := range values {
		stringValue, isString := item.(string)
		if isString {
			result = append(result, stringValue)
		}
	}
	return result
}

func (service *Service) blueclawJSONRequest(ctx context.Context, method string, path string, body any, responseValue any) error {
	var reader io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return errorValue
		}
		reader = strings.NewReader(string(document))
	}
	requestURL := strings.TrimRight(service.Configuration.BlueclawBaseURL, "/") + path
	request, errorValue := http.NewRequestWithContext(ctx, method, requestURL, reader)
	if errorValue != nil {
		return errorValue
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if responseValue != nil {
			return json.NewDecoder(response.Body).Decode(responseValue)
		}
		return nil
	}
	responseDocument, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("Blueclaw %s %s returned %d: %s", method, path, response.StatusCode, strings.TrimSpace(string(responseDocument)))
}

func (service *Service) triggerUsersSync(ctx context.Context) {
	if service.RunCommand == nil {
		if _, errorValue := exec.LookPath("systemctl"); errorValue != nil {
			return
		}
	}
	if _, errorValue := service.runCommand(ctx, "systemctl", "start", "internkim-users-sync.service"); errorValue != nil {
		log.Printf("users sync trigger failed: %v", errorValue)
	}
}

func (service *Service) firstAdminBootstrapPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "first-admin-bootstrap.json")
}

func (service *Service) readFirstAdminBootstrapResult() firstAdminBootstrapResult {
	document, errorValue := os.ReadFile(service.firstAdminBootstrapPath())
	if errorValue != nil {
		return firstAdminBootstrapResult{}
	}
	var result firstAdminBootstrapResult
	if errorValue := json.Unmarshal(document, &result); errorValue != nil {
		return firstAdminBootstrapResult{}
	}
	return result
}

func (service *Service) writeFirstAdminBootstrapResult(result firstAdminBootstrapResult) firstAdminBootstrapResult {
	if result.Status == "" {
		result.Status = firstAdminBootstrapPending
	}
	if result.Status == firstAdminBootstrapIdentityMissing || result.Status == firstAdminBootstrapRejected {
		return result
	}
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		log.Printf("first admin bootstrap status write failed: %v", errorValue)
		return result
	}
	document, errorValue := json.Marshal(result)
	if errorValue != nil {
		log.Printf("first admin bootstrap status marshal failed: %v", errorValue)
		return result
	}
	if errorValue := os.WriteFile(service.firstAdminBootstrapPath(), document, 0o600); errorValue != nil {
		log.Printf("first admin bootstrap status write failed: %v", errorValue)
	}
	return result
}

func (service *Service) firstAdminPasswordPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "first-admin-password.json")
}

func (service *Service) writeFirstAdminPassword(email string, password string) error {
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		return errorValue
	}
	document, errorValue := json.Marshal(firstAdminPasswordDocument{
		Email:    strings.ToLower(strings.TrimSpace(email)),
		Password: password,
	})
	if errorValue != nil {
		return errorValue
	}
	return os.WriteFile(service.firstAdminPasswordPath(), document, 0o600)
}

func (service *Service) consumeFirstAdminPassword(email string) firstAdminPasswordDocument {
	path := service.firstAdminPasswordPath()
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return firstAdminPasswordDocument{}
	}
	var passwordDocument firstAdminPasswordDocument
	if errorValue := json.Unmarshal(document, &passwordDocument); errorValue != nil {
		return firstAdminPasswordDocument{}
	}
	if !strings.EqualFold(passwordDocument.Email, email) {
		return firstAdminPasswordDocument{}
	}
	_ = os.Remove(path)
	return passwordDocument
}

func (service *Service) writeUserRole(ctx context.Context, fleetID string, fleetSecret string, email string, role string) error {
	payload := map[string]string{
		"fleet_id": fleetID,
		"email":    strings.ToLower(strings.TrimSpace(email)),
		"role":     normalizeAdminUserRole(role),
		"handle":   normalizeMemberHandle(memberHandleBase(email)),
		"name":     firstNonEmpty(strings.TrimSpace(email), "Admin"),
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.APIBaseURL, "/") + "/api/users"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, strings.NewReader(string(document)))
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-INTERNKIM-FLEET-ID", fleetID)
	request.Header.Set("X-INTERNKIM-FLEET-SECRET", fleetSecret)
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		return nil
	}
	responseDocument, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return fmt.Errorf("write user role returned %d: %s", response.StatusCode, strings.TrimSpace(string(responseDocument)))
}

func (service *Service) writeClaimedAdminRole(ctx context.Context, email string) error {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		return fmt.Errorf("device auth is not configured")
	}
	return service.writeUserRole(ctx, fleetID, fleetSecret, email, "admin")
}

func (service *Service) currentUserRecords(ctx context.Context) ([]adminUserMutation, error) {
	if service.deviceBelongsToACompany() {
		return service.companyMemberRecords(ctx)
	}
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID == "" || fleetSecret == "" {
		return nil, fmt.Errorf("device auth is not configured")
	}
	return service.lookupUserRecords(ctx, fleetID, fleetSecret)
}

func (service *Service) hasCurrentAdminUsers(ctx context.Context) (bool, error) {
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	for _, record := range records {
		if record.Role == "admin" {
			return true, nil
		}
	}
	return false, nil
}

func (service *Service) isCurrentAdminEmail(ctx context.Context, callerEmail string) bool {
	if strings.TrimSpace(callerEmail) == "" {
		return false
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return false
	}
	for _, record := range records {
		if record.Role == "admin" && strings.EqualFold(record.Email, callerEmail) {
			return true
		}
	}
	return false
}

func (service *Service) currentAdminUserRole(ctx context.Context, callerEmail string) string {
	if strings.TrimSpace(callerEmail) == "" {
		return adminUserRoleMember
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return adminUserRoleMember
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, callerEmail) {
			return normalizeAdminUserRole(record.Role)
		}
	}
	return adminUserRoleMember
}
