package admind

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func (service *Service) organizationProfilesOfTheCompany(ctx context.Context) ([]organizationProfile, error) {
	members, errorValue := service.companyMembers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	memberIDByEmail := map[string]string{}
	for _, member := range members {
		memberIDByEmail[normalizedRosterEmail(member.Email)] = strings.TrimSpace(member.MemberID)
	}
	profiles := make([]organizationProfile, 0, len(members))
	for _, member := range members {
		profiles = append(profiles, organizationProfile{
			MemberID:     strings.TrimSpace(member.MemberID),
			Email:        normalizedRosterEmail(member.Email),
			JobTitle:     strings.TrimSpace(member.JobTitle),
			GroupID:      strings.TrimSpace(member.TeamID),
			PhoneNumber:  strings.TrimSpace(member.PhoneNumber),
			HireDate:     strings.TrimSpace(member.HireDate),
			SupervisorID: memberIDByEmail[normalizedRosterEmail(member.SupervisorEmail)],
			Status:       strings.TrimSpace(member.Status),
		})
	}
	return profiles, nil
}

func (service *Service) organizationGroupsOfTheCompany(ctx context.Context, requesterEmail string) ([]orgGroupRecord, error) {
	client := service.centralPlane()
	if client == nil {
		return nil, errNoCompanyDirectory
	}
	teams, errorValue := client.Teams(ctx, requesterEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	groups := make([]orgGroupRecord, 0, len(teams))
	for _, team := range teams {
		groups = append(groups, orgGroupRecord{
			ID:       strings.TrimSpace(team.TeamID),
			Name:     strings.TrimSpace(team.Name),
			ParentID: strings.TrimSpace(team.ParentTeamID),
		})
	}
	return groups, nil
}

func (service *Service) companyMembers(ctx context.Context) ([]centralplane.Member, error) {
	client := service.centralPlane()
	if client == nil {
		return nil, errNoCompanyDirectory
	}
	return client.Members(ctx)
}

func (service *Service) organizationRecordsOfTheCompany(ctx context.Context) ([]adminUserMutation, error) {
	members, errorValue := service.companyMembers(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	memberIDByEmail := map[string]string{}
	for _, member := range members {
		memberIDByEmail[normalizedRosterEmail(member.Email)] = strings.TrimSpace(member.MemberID)
	}
	records := make([]adminUserMutation, 0, len(members))
	for _, member := range members {
		if member.HasLeftTheCompany() {
			continue
		}
		records = append(records, adminUserMutation{
			MemberID:     strings.TrimSpace(member.MemberID),
			Handle:       handleFromEmail(member.Email),
			Name:         strings.TrimSpace(member.Name),
			Email:        normalizedRosterEmail(member.Email),
			HireDate:     strings.TrimSpace(member.HireDate),
			Role:         normalizeAdminUserRole(member.Role),
			Circles:      member.Circles,
			JobTitle:     strings.TrimSpace(member.JobTitle),
			GroupID:      strings.TrimSpace(member.TeamID),
			PhoneNumber:  strings.TrimSpace(member.PhoneNumber),
			SupervisorID: memberIDByEmail[normalizedRosterEmail(member.SupervisorEmail)],
			Status:       strings.TrimSpace(member.Status),
		})
	}
	return records, nil
}

func (service *Service) persistOrganizationHireDate(ctx context.Context, email string, hireDate string) {
	normalizedEmail := normalizedRosterEmail(email)
	normalizedHireDate := strings.TrimSpace(hireDate)
	if normalizedEmail == "" || normalizedHireDate == "" {
		return
	}
	client := service.centralPlane()
	if client == nil {
		return
	}
	if _, errorValue := client.WriteOrganizationProfiles(ctx, []centralplane.OrganizationProfile{{
		Email:    normalizedEmail,
		HireDate: normalizedHireDate,
	}}); errorValue != nil {
		slog.WarnContext(ctx, "the hire date could not be written to the company directory",
			"email", normalizedEmail, "error", errorValue)
	}
}

// A record tool is invoked as a person. A colleague asking reads it as
// themselves; a read nobody asked for reads it as the administrator who
// claimed the device.
func (service *Service) recordReaderEmail(request *http.Request) string {
	if email := strings.ToLower(strings.TrimSpace(service.webActorEmail(request))); email != "" {
		return email
	}
	return service.claimedAdminEmail()
}
