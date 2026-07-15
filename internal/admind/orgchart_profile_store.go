package admind

import (
	"context"
	"time"
)

func (service *Service) writeOrgchartProfiles(ctx context.Context, profiles []orgchartProfile) error {
	if len(profiles) == 0 {
		return nil
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	writtenProfiles := make([]orgchartProfile, 0, len(profiles))
	for _, profile := range profiles {
		normalizedProfile := normalizeOrgchartProfile(profile)
		profileKey := orgchartProfileKey(normalizedProfile)
		if profileKey == "" {
			continue
		}
		groupIDs, errorValue := encodeOrgchartStringList(normalizedProfile.GroupIDs)
		if errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		projectIDs, errorValue := encodeOrgchartStringList(normalizedProfile.ProjectIDs)
		if errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `
DELETE FROM orgchart_profiles
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
INSERT INTO orgchart_profiles(
	profile_key,
	user_id,
	email,
	job_title,
	position_level,
	primary_group_id,
	group_ids,
	supervisor_id,
	project_ids,
	team_role,
	employment_status,
	is_orgchart_visible,
	updated_at
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(profile_key) DO UPDATE SET
	user_id = excluded.user_id,
	email = excluded.email,
	job_title = excluded.job_title,
	position_level = excluded.position_level,
	primary_group_id = excluded.primary_group_id,
	group_ids = excluded.group_ids,
	supervisor_id = excluded.supervisor_id,
	project_ids = excluded.project_ids,
	team_role = excluded.team_role,
	employment_status = excluded.employment_status,
	is_orgchart_visible = excluded.is_orgchart_visible,
	updated_at = excluded.updated_at`,
			profileKey,
			normalizedProfile.UserID,
			normalizedProfile.Email,
			normalizedProfile.JobTitle,
			normalizedProfile.PositionLevel,
			normalizedProfile.PrimaryGroupID,
			groupIDs,
			normalizedProfile.SupervisorID,
			projectIDs,
			normalizedProfile.TeamRole,
			normalizedProfile.EmploymentStatus,
			boolToSQLiteInt(normalizedProfile.IsOrgchartVisible),
			time.Now().UTC().Format(time.RFC3339),
		); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
		writtenProfiles = append(writtenProfiles, normalizedProfile)
	}
	if errorValue := invalidateOrgchartProfiles(ctx, transaction, writtenProfiles); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) readOrgchartProfilesByEmail(ctx context.Context) (map[string]orgchartProfile, error) {
	profiles, errorValue := service.readOrgchartProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByEmail := map[string]orgchartProfile{}
	for _, profile := range profiles {
		if profile.Email != "" {
			profilesByEmail[profile.Email] = profile
		}
	}
	return profilesByEmail, nil
}

func (service *Service) readOrgchartProfilesByUserID(ctx context.Context) (map[string]orgchartProfile, error) {
	profiles, errorValue := service.readOrgchartProfiles(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	profilesByUserID := map[string]orgchartProfile{}
	for _, profile := range profiles {
		if profile.UserID != "" {
			profilesByUserID[profile.UserID] = profile
		}
	}
	return profilesByUserID, nil
}

func (service *Service) readOrgchartProfiles(ctx context.Context) ([]orgchartProfile, error) {
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT user_id, email, job_title, position_level, primary_group_id, group_ids, supervisor_id, project_ids, team_role, employment_status, is_orgchart_visible
FROM orgchart_profiles
ORDER BY position_level, email`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	profiles := []orgchartProfile{}
	for rows.Next() {
		var profile orgchartProfile
		var groupIDs string
		var projectIDs string
		var isOrgchartVisible int
		if errorValue := rows.Scan(
			&profile.UserID,
			&profile.Email,
			&profile.JobTitle,
			&profile.PositionLevel,
			&profile.PrimaryGroupID,
			&groupIDs,
			&profile.SupervisorID,
			&projectIDs,
			&profile.TeamRole,
			&profile.EmploymentStatus,
			&isOrgchartVisible,
		); errorValue != nil {
			return nil, errorValue
		}
		profile.GroupIDs = decodeOrgchartStringList(groupIDs)
		profile.ProjectIDs = decodeOrgchartStringList(projectIDs)
		profile.IsOrgchartVisible = isOrgchartVisible == 1
		profiles = append(profiles, normalizeOrgchartProfile(profile))
	}
	return profiles, rows.Err()
}
