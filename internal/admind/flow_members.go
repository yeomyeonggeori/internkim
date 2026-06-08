package admind

import (
	"net/http"
	"sort"
	"strings"
)

func (service *Service) flowMembers(request *http.Request) []flowMember {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID != "" && fleetSecret != "" {
		if records, errorValue := service.lookupUserRecords(request.Context(), fleetID, fleetSecret); errorValue == nil && len(records) > 0 {
			return membersFromUserRecords(records)
		}
	}
	return nil
}

func membersFromUserRecords(records []adminUserMutation) []flowMember {
	members := make([]flowMember, 0, len(records))
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
		members = append(members, flowMember{
			ID:                 stableFlowID(email),
			Name:               name,
			Email:              email,
			MattermostUsername: strings.TrimSpace(firstNonEmpty(record.MattermostUsername, record.Handle)),
			HireDate:           strings.TrimSpace(record.HireDate),
			Role:               normalizeAdminUserRole(record.Role),
			MattermostStatus:   firstNonEmpty(record.Status, "active"),
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

func resolveCurrentUserName(members []flowMember, email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	for _, member := range members {
		if strings.EqualFold(strings.TrimSpace(member.Email), normalized) {
			return member.Name
		}
	}
	return strings.TrimSuffix(normalized, "@"+emailDomain(normalized))
}

func memberIDs(members []flowMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

func memberNames(members []flowMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.Name)
	}
	return values
}

func calculateFlowMemberDistances(members []flowMember, tasks []flowTask, definitions flowDefinitions) []flowMember {
	result := append([]flowMember(nil), members...)
	memberIndex := map[string]int{}
	for index, member := range result {
		memberIndex[member.ID] = index
	}
	for _, task := range tasks {
		for _, memberID := range task.ParticipantIDs {
			index, found := memberIndex[memberID]
			if !found {
				continue
			}
			if isFlowCompletedStatus(task.Status) {
				result[index].CompleteTaskCount++
			} else if !isFlowInactiveStatus(task.Status) {
				result[index].ActiveTaskCount++
			}
			result[index].Distance += progressDistanceForTask(task, definitions)
			result[index].Score = result[index].Distance
		}
	}
	return result
}
