package admind

import (
	"net/http"
	"strings"
)

func (service *Service) attendanceMembersForSummary(request *http.Request, actorEmail string, isAdmin bool, teamVisible bool) []attendanceMember {
	normalizedActorEmail := strings.ToLower(strings.TrimSpace(actorEmail))
	if !isAdmin && !teamVisible {
		if normalizedActorEmail == "" {
			return []attendanceMember{}
		}
		return []attendanceMember{{
			Email:       normalizedActorEmail,
			DisplayName: strings.TrimSuffix(normalizedActorEmail, "@"+emailDomain(normalizedActorEmail)),
		}}
	}
	records, found := service.attendanceUserRecordsForMembers(request)
	if !found {
		return []attendanceMember{}
	}
	members := attendanceMembersFromAdminUserRecords(records)
	return members
}

func (service *Service) attendanceUserRecordsForMembers(request *http.Request) ([]adminUserMutation, bool) {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID != "" && fleetSecret != "" {
		records, errorValue := service.lookupUserRecords(request.Context(), fleetID, fleetSecret)
		if errorValue != nil {
			return nil, false
		}
		return records, true
	}
	response, errorValue := service.buildLocalUsersResponse(request.Context())
	if errorValue != nil {
		return nil, false
	}
	return response.Records, true
}

func attendanceMembersFromAdminUserRecords(records []adminUserMutation) []attendanceMember {
	members := membersFromUserRecords(records)
	result := make([]attendanceMember, 0, len(members))
	for _, flowMember := range members {
		email := strings.ToLower(strings.TrimSpace(flowMember.Email))
		if email == "" {
			continue
		}
		displayName := strings.TrimSpace(flowMember.Name)
		if displayName == "" {
			displayName = email
		}
		result = append(result, attendanceMember{
			Email:              email,
			DisplayName:        displayName,
			Image:              flowMember.Image,
			MattermostUsername: strings.TrimSpace(flowMember.MattermostUsername),
		})
	}
	return result
}
