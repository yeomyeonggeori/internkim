package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

type rosterReadiness struct {
	AppliedAt   time.Time
	LastFailure string
}

func (service *Service) recordRosterReconcile(errorValue error) {
	service.rosterReadinessMutex.Lock()
	defer service.rosterReadinessMutex.Unlock()
	if errorValue != nil {
		service.rosterReadiness.LastFailure = errorValue.Error()
		return
	}
	service.rosterReadiness = rosterReadiness{AppliedAt: time.Now().UTC()}
}

func (service *Service) currentRosterReadiness() rosterReadiness {
	service.rosterReadinessMutex.Lock()
	defer service.rosterReadinessMutex.Unlock()
	return service.rosterReadiness
}

func (service *Service) writeRosterReadiness(responseWriter http.ResponseWriter) {
	readiness := service.currentRosterReadiness()
	if readiness.AppliedAt.IsZero() {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(responseWriter).Encode(map[string]any{"status": "waiting", "lastFailure": readiness.LastFailure})
		return
	}
	service.writeJSON(responseWriter, map[string]any{"status": "applied", "appliedAt": readiness.AppliedAt})
}

func (service *Service) applyRoster(ctx context.Context, members []centralplane.Member) error {
	errorValue := service.deliverRosterReconciledWith(ctx, rosterRecordsOfMembers(members))
	service.recordRosterReconcile(errorValue)
	return errorValue
}

func rosterRecordsOfMembers(members []centralplane.Member) []adminUserMutation {
	records := make([]adminUserMutation, 0, len(members))
	for _, member := range members {
		if !member.IsWorking {
			continue
		}
		records = append(records, adminUserMutation{
			MemberID: strings.TrimSpace(member.MemberID),
			Email:    strings.ToLower(strings.TrimSpace(member.Email)),
			Name:     strings.TrimSpace(member.Name),
			Role:     strings.TrimSpace(member.Role),
			Circles:  member.Circles,
		})
	}
	return records
}

func (service *Service) deliverRosterReconciledWith(ctx context.Context, records []adminUserMutation) error {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	servedRoster, errorValue := json.Marshal(policyDocument)
	if errorValue != nil {
		return errorValue
	}
	reconcileRosterPeople(policyDocument, records, service.alwaysRetainedRosterEmails(), service.workspaceLanguage(ctx))
	service.reconcileCompanySnapshot(ctx, policyDocument)
	reconciledRoster, errorValue := json.Marshal(policyDocument)
	if errorValue != nil {
		return errorValue
	}
	if string(servedRoster) == string(reconciledRoster) {
		return nil
	}
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}

func (service *Service) alwaysRetainedRosterEmails() []string {
	return []string{service.seedAdminEmail(), service.claimedAdminEmail()}
}

func reconcileRosterPeople(policyDocument map[string]any, records []adminUserMutation, retainedEmails []string, language string) {
	adoptRosterRecords(policyDocument, records, language)
	dropRosterPeopleTheDirectoryNoLongerKnows(policyDocument, rosterDirectoryEmails(records), rosterEmailSet(retainedEmails))
	declareTheCirclesPeopleHold(policyDocument)
}

func adoptRosterRecords(policyDocument map[string]any, records []adminUserMutation, language string) {
	people, _ := policyDocument["people"].([]any)
	knownCount := len(people)
	for _, record := range records {
		if !isAdoptableRosterRecord(record) {
			continue
		}
		email := normalizedRosterEmail(record.Email)
		person := blueclawPersonWithEmail(people, email)
		if person == nil {
			person = map[string]any{"personID": strings.TrimSpace(record.MemberID), "emails": []any{email}}
			people = append(people, person)
		}
		applyBlueclawPersonAttributes(person, record.Name, record.Role, record.Circles, nil, language)
	}
	if len(people) != knownCount {
		policyDocument["people"] = people
	}
}

func dropRosterPeopleTheDirectoryNoLongerKnows(policyDocument map[string]any, directoryEmails map[string]bool, retainedEmails map[string]bool) {
	people, _ := policyDocument["people"].([]any)
	if len(people) == 0 || len(directoryEmails) == 0 {
		return
	}
	remaining := make([]any, 0, len(people))
	for _, value := range people {
		if isRosterPersonRetained(value, directoryEmails, retainedEmails) {
			remaining = append(remaining, value)
		}
	}
	if len(remaining) != len(people) {
		policyDocument["people"] = remaining
	}
}

func isRosterPersonRetained(value any, directoryEmails map[string]bool, retainedEmails map[string]bool) bool {
	person, isPerson := value.(map[string]any)
	if !isPerson {
		return true
	}
	emails := rosterPersonEmails(person)
	if len(emails) == 0 {
		return true
	}
	for _, email := range emails {
		if directoryEmails[email] || retainedEmails[email] {
			return true
		}
	}
	return false
}

func isAdoptableRosterRecord(record adminUserMutation) bool {
	return strings.TrimSpace(record.MemberID) != "" && normalizedRosterEmail(record.Email) != ""
}

func rosterDirectoryEmails(records []adminUserMutation) map[string]bool {
	emails := map[string]bool{}
	for _, record := range records {
		if !isAdoptableRosterRecord(record) {
			continue
		}
		emails[normalizedRosterEmail(record.Email)] = true
	}
	return emails
}

func rosterEmailSet(emails []string) map[string]bool {
	set := map[string]bool{}
	for _, email := range emails {
		normalizedEmail := normalizedRosterEmail(email)
		if normalizedEmail == "" {
			continue
		}
		set[normalizedEmail] = true
	}
	return set
}

func rosterPersonEmails(person map[string]any) []string {
	emailValues, _ := person["emails"].([]any)
	emails := make([]string, 0, len(emailValues))
	for _, value := range emailValues {
		candidate, isString := value.(string)
		if !isString {
			continue
		}
		normalizedEmail := normalizedRosterEmail(candidate)
		if normalizedEmail == "" {
			continue
		}
		emails = append(emails, normalizedEmail)
	}
	return emails
}

func normalizedRosterEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
