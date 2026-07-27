package admind

import (
	"context"
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
		projectIDs, errorValue := encodeOrganizationStringList(normalizedProfile.ProjectIDs)
		if errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
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
			normalizedProfile.UserID,
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
	position_level,
	group_id,
	phone_number,
	supervisor_id,
	project_ids,
	team_role,
	employment_status,
	is_organization_visible,
	updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(profile_key) DO UPDATE SET
	user_id = excluded.user_id,
	email = excluded.email,
	job_title = excluded.job_title,
	position_level = excluded.position_level,
	group_id = excluded.group_id,
	phone_number = excluded.phone_number,
	supervisor_id = excluded.supervisor_id,
	project_ids = excluded.project_ids,
	team_role = excluded.team_role,
	employment_status = excluded.employment_status,
	is_organization_visible = excluded.is_organization_visible,
	updated_at = excluded.updated_at`,
			profileKey,
			normalizedProfile.UserID,
			normalizedProfile.Email,
			normalizedProfile.JobTitle,
			normalizedProfile.PositionLevel,
			normalizedProfile.GroupID,
			normalizedProfile.PhoneNumber,
			normalizedProfile.SupervisorID,
			projectIDs,
			normalizedProfile.TeamRole,
			normalizedProfile.EmploymentStatus,
			boolToSQLiteInt(normalizedProfile.IsOrganizationVisible),
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
		if profile.UserID != "" {
			profilesByUserID[profile.UserID] = profile
		}
	}
	return profilesByUserID, nil
}

func (service *Service) readOrganizationProfiles(ctx context.Context) ([]organizationProfile, error) {
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT user_id, email, job_title, position_level, group_id, phone_number, supervisor_id, project_ids, team_role, employment_status, is_organization_visible
FROM organization_profiles
ORDER BY position_level, email`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	profiles := []organizationProfile{}
	for rows.Next() {
		var profile organizationProfile
		var projectIDs string
		var isOrganizationVisible int
		if errorValue := rows.Scan(
			&profile.UserID,
			&profile.Email,
			&profile.JobTitle,
			&profile.PositionLevel,
			&profile.GroupID,
			&profile.PhoneNumber,
			&profile.SupervisorID,
			&projectIDs,
			&profile.TeamRole,
			&profile.EmploymentStatus,
			&isOrganizationVisible,
		); errorValue != nil {
			return nil, errorValue
		}
		profile.ProjectIDs = decodeOrganizationStringList(projectIDs)
		profile.IsOrganizationVisible = isOrganizationVisible == 1
		profiles = append(profiles, normalizeOrganizationProfile(profile))
	}
	return profiles, rows.Err()
}
