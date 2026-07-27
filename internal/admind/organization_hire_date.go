package admind

import (
	"context"
	"log"
	"strings"
)

func (service *Service) persistOrganizationHireDate(ctx context.Context, userID string, email string, hireDate string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedEmail == "" && normalizedUserID == "" {
		return
	}
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return
	}
	profile, changed := organizationProfileWithHireDate(profiles, normalizedUserID, normalizedEmail, hireDate)
	if !changed {
		return
	}
	_ = service.writeOrganizationProfiles(ctx, []organizationProfile{profile})
}

func (service *Service) adoptAccountHireDates(ctx context.Context) {
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return
	}
	response, errorValue := service.loadOrganizationUserListSource(ctx)
	if errorValue != nil {
		return
	}
	adopted := []organizationProfile{}
	for _, record := range response.Records {
		if strings.TrimSpace(record.HireDate) == "" {
			continue
		}
		profile, changed := organizationProfileWithHireDate(profiles, record.UserID, record.Email, record.HireDate)
		if changed && profileHireDateIsEmpty(profiles, record.UserID, record.Email) {
			adopted = append(adopted, profile)
		}
	}
	if len(adopted) == 0 {
		return
	}
	if errorValue := service.writeOrganizationProfiles(ctx, adopted); errorValue != nil {
		log.Printf("organization hire date adoption failed: %v", errorValue)
		return
	}
	log.Printf("organization hire date adoption completed: adopted=%d", len(adopted))
}

func organizationProfileWithHireDate(profiles []organizationProfile, userID string, email string, hireDate string) (organizationProfile, bool) {
	profilesByUserID, profilesByEmail := organizationProfileIndexes(profiles)
	profile, found := organizationProfileForUser(adminUserMutation{UserID: userID, Email: email}, profilesByUserID, profilesByEmail)
	if !found {
		profile = organizationProfile{EmploymentStatus: organizationEmploymentStatusActive, IsOrganizationVisible: true}
	}
	profile.UserID = firstNonEmpty(userID, profile.UserID)
	profile.Email = firstNonEmpty(strings.ToLower(strings.TrimSpace(email)), profile.Email)
	normalizedHireDate := strings.TrimSpace(hireDate)
	if profile.HireDate == normalizedHireDate {
		return profile, false
	}
	profile.HireDate = normalizedHireDate
	return profile, true
}

func profileHireDateIsEmpty(profiles []organizationProfile, userID string, email string) bool {
	profilesByUserID, profilesByEmail := organizationProfileIndexes(profiles)
	profile, found := organizationProfileForUser(adminUserMutation{UserID: userID, Email: email}, profilesByUserID, profilesByEmail)
	return !found || strings.TrimSpace(profile.HireDate) == ""
}
