package admind

import (
	"context"
	"strings"
	"time"
)

func (service *Service) writeOrganizationProfiles(ctx context.Context, profiles []organizationProfile) error {
	if len(profiles) == 0 {
		return nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	writtenProfiles := make([]organizationProfile, 0, len(profiles))
	for _, profile := range profiles {
		normalizedProfile := normalizeOrganizationProfile(profile)
		profileKey := organizationProfileKey(normalizedProfile)
		if profileKey == "" {
			continue
		}
		if _, errorValue := transaction.ExecContext(ctx, `
DELETE FROM organization_profiles
WHERE profile_key != ?
	AND (
		(email != '' AND email = ?)
		OR (user_id != '' AND user_id = ?)
	)`,
			profileKey,
			normalizedProfile.Email,
			normalizedProfile.MemberID,
		); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO organization_profiles(
	profile_key,
	user_id,
	email,
	job_title,
	group_id,
	phone_number,
	hire_date,
	supervisor_id,
	status,
	updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(profile_key) DO UPDATE SET
	user_id = excluded.user_id,
	email = excluded.email,
	job_title = excluded.job_title,
	group_id = excluded.group_id,
	phone_number = excluded.phone_number,
	hire_date = excluded.hire_date,
	supervisor_id = excluded.supervisor_id,
	status = excluded.status,
	updated_at = excluded.updated_at`,
			profileKey,
			normalizedProfile.MemberID,
			normalizedProfile.Email,
			normalizedProfile.JobTitle,
			normalizedProfile.GroupID,
			normalizedProfile.PhoneNumber,
			normalizedProfile.HireDate,
			normalizedProfile.SupervisorID,
			normalizedProfile.Status,
			time.Now().UTC().Format(time.RFC3339),
		); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		writtenProfiles = append(writtenProfiles, normalizedProfile)
	}
	if errorValue := invalidateOrganizationProfiles(ctx, transaction, writtenProfiles); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) readOrganizationProfilesByEmail(ctx context.Context) (map[string]organizationProfile, error) {
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByEmail := map[string]organizationProfile{}
	for _, profile := range profiles {
		if profile.Email != "" {
			profilesByEmail[profile.Email] = profile
		}
	}
	return profilesByEmail, nil
}

func (service *Service) readOrganizationProfilesByUserID(ctx context.Context) (map[string]organizationProfile, error) {
	profiles, errorValue := service.readOrganizationProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByUserID := map[string]organizationProfile{}
	for _, profile := range profiles {
		if profile.MemberID != "" {
			profilesByUserID[profile.MemberID] = profile
		}
	}
	return profilesByUserID, nil
}

// A job title, a supervisor and a phone number describe somebody who works
// here. When they no longer do, the description is not a record of anything
// that happened - the attendance and the leave are that - so it goes with them.
func (service *Service) forgetOrganizationProfile(ctx context.Context, email string, memberID string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	normalizedMemberID := strings.TrimSpace(memberID)
	if normalizedEmail == "" && normalizedMemberID == "" {
		return nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
DELETE FROM organization_profiles
WHERE (email != '' AND email = ?)
	OR (user_id != '' AND user_id = ?)`,
		normalizedEmail,
		normalizedMemberID,
	)
	return errorValue
}

func (service *Service) readOrganizationProfiles(ctx context.Context) ([]organizationProfile, error) {
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT user_id, email, job_title, group_id, phone_number, hire_date, supervisor_id, status
FROM organization_profiles
ORDER BY email`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	profiles := []organizationProfile{}
	for rows.Next() {
		var profile organizationProfile
		if errorValue := rows.Scan(
			&profile.MemberID,
			&profile.Email,
			&profile.JobTitle,
			&profile.GroupID,
			&profile.PhoneNumber,
			&profile.HireDate,
			&profile.SupervisorID,
			&profile.Status,
		); errorValue != nil {
			return nil, errorValue
		}
		profiles = append(profiles, normalizeOrganizationProfile(profile))
	}
	return profiles, rows.Err()
}
