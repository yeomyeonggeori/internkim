package admind

import (
	"context"
	"net/http"
	"sort"
	"strings"
)

func (service *Service) taskMembers(request *http.Request) []taskMember {
	return membersFromUserRecords(service.organizationChartUserRecords(request))
}

func (service *Service) organizationChartUserRecords(request *http.Request) []adminUserMutation {
	records, errorValue := service.organizationRecordsOfTheCompany(request.Context())
	if errorValue != nil {
		return service.accountDirectoryUserRecords(request)
	}
	return records
}

func (service *Service) accountDirectoryUserRecords(request *http.Request) []adminUserMutation {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	var records []adminUserMutation
	if fleetID != "" && fleetSecret != "" {
		if fetched, errorValue := service.lookupUserRecords(request.Context(), fleetID, fleetSecret); errorValue == nil {
			records = fetched
		}
	}
	return mergeUserRecordsByEmail(records, service.blueclawPolicyUserRecords(request.Context()))
}

func (service *Service) blueclawPolicyUserRecords(ctx context.Context) []adminUserMutation {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return service.cachedPolicyUserRecords()
	}
	people, _ := policyDocument["people"].([]any)
	records := []adminUserMutation{}
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		name := strings.TrimSpace(policyString(person["displayName"]))
		jobTitle := strings.TrimSpace(policyString(person["jobTitle"]))
		emailValues, _ := person["emails"].([]any)
		for _, emailValue := range emailValues {
			email := strings.ToLower(strings.TrimSpace(policyString(emailValue)))
			if email == "" {
				continue
			}
			records = append(records, adminUserMutation{Email: email, Name: name, JobTitle: jobTitle, Status: "active"})
		}
	}
	service.storePolicyUserRecords(records)
	return records
}

func (service *Service) cachedPolicyUserRecords() []adminUserMutation {
	service.policyRecordCacheMutex.Lock()
	defer service.policyRecordCacheMutex.Unlock()
	return append([]adminUserMutation{}, service.policyRecordCache...)
}

func (service *Service) storePolicyUserRecords(records []adminUserMutation) {
	service.policyRecordCacheMutex.Lock()
	defer service.policyRecordCacheMutex.Unlock()
	service.policyRecordCache = append([]adminUserMutation{}, records...)
}

func mergeUserRecordsByEmail(primaryRecords []adminUserMutation, additionalRecords []adminUserMutation) []adminUserMutation {
	mergedRecords := []adminUserMutation{}
	seenEmail := map[string]bool{}
	for _, recordGroup := range [][]adminUserMutation{primaryRecords, additionalRecords} {
		for _, record := range recordGroup {
			email := strings.ToLower(strings.TrimSpace(record.Email))
			if email == "" || seenEmail[email] {
				continue
			}
			seenEmail[email] = true
			mergedRecords = append(mergedRecords, record)
		}
	}
	return mergedRecords
}

func membersFromUserRecords(records []adminUserMutation) []taskMember {
	members := make([]taskMember, 0, len(records))
	for _, record := range records {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if email == "" {
			continue
		}
		name := strings.TrimSpace(record.Name)
		if name == "" {
			name = strings.TrimSpace(record.MattermostUsername)
		}
		if name == "" {
			name = strings.TrimSuffix(email, "@"+emailDomain(email))
		}
		id := stableTaskID(email)
		members = append(members, taskMember{
			ID:                 id,
			Name:               name,
			Email:              email,
			MattermostUsername: strings.TrimSpace(firstNonEmpty(record.MattermostUsername, record.Handle)),
			HireDate:           strings.TrimSpace(record.HireDate),
			Role:               normalizeAdminUserRole(record.Role),
		})
	}
	sort.Slice(members, func(leftIndex int, rightIndex int) bool {
		leftMember := members[leftIndex]
		rightMember := members[rightIndex]
		if leftMember.HireDate != "" || rightMember.HireDate != "" {
			if leftMember.HireDate == "" {
				return false
			}
			if rightMember.HireDate == "" {
				return true
			}
			if leftMember.HireDate != rightMember.HireDate {
				return leftMember.HireDate < rightMember.HireDate
			}
		}
		if leftMember.Name != rightMember.Name {
			return strings.ToLower(leftMember.Name) < strings.ToLower(rightMember.Name)
		}
		return leftMember.Email < rightMember.Email
	})
	return members
}


func memberIDs(members []taskMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

func memberNames(members []taskMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.Name)
	}
	return values
}
