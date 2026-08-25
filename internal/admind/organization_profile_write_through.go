package admind

import (
	"context"
	"log"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func (service *Service) writeOrganizationProfilesToTheDirectory(ctx context.Context, profiles []organizationProfile) error {
	client := service.centralPlane()
	if client == nil {
		return nil
	}
	offered, errorValue := service.directoryProfilesFor(ctx, profiles)
	if errorValue != nil {
		return errorValue
	}
	written, errorValue := client.WriteOrganizationProfiles(ctx, offered)
	if errorValue != nil {
		return errorValue
	}
	log.Printf("organization profiles written to the company directory: %d of %d", len(written), len(offered))
	return nil
}

func (service *Service) directoryProfilesFor(ctx context.Context, profiles []organizationProfile) ([]centralplane.OrganizationProfile, error) {
	emailByMemberID, errorValue := service.organizationEmailByMemberID(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	groupNameByID, errorValue := service.organizationGroupNameByID(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	offered := make([]centralplane.OrganizationProfile, 0, len(profiles))
	for _, profile := range profiles {
		normalized := normalizeOrganizationProfile(profile)
		if normalized.Email == "" {
			continue
		}
		offered = append(offered, centralplane.OrganizationProfile{
			Email:           normalized.Email,
			JobTitle:        normalized.JobTitle,
			PhoneNumber:     normalized.PhoneNumber,
			HireDate:        normalized.HireDate,
			SupervisorEmail: emailByMemberID[normalized.SupervisorID],
			TeamName:        groupNameByID[normalized.GroupID],
		})
	}
	return offered, nil
}

func (service *Service) organizationEmailByMemberID(ctx context.Context) (map[string]string, error) {
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	emailByMemberID := map[string]string{}
	for _, profile := range profiles {
		normalized := normalizeOrganizationProfile(profile)
		if normalized.MemberID != "" && normalized.Email != "" {
			emailByMemberID[normalized.MemberID] = normalized.Email
		}
	}
	return emailByMemberID, nil
}

func (service *Service) organizationGroupNameByID(ctx context.Context) (map[string]string, error) {
	groups, errorValue := service.readOrganizationGroups(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	nameByID := map[string]string{}
	for _, group := range groups {
		nameByID[strings.TrimSpace(group.ID)] = strings.TrimSpace(group.Name)
	}
	return nameByID, nil
}
