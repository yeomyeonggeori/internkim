package admind

import (
	"encoding/json"
	"strings"
)

func normalizeOrganizationProfile(profile organizationProfile) organizationProfile {
	profile.UserID = strings.TrimSpace(profile.UserID)
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.JobTitle = strings.TrimSpace(profile.JobTitle)
	profile.GroupID = strings.TrimSpace(profile.GroupID)
	profile.SupervisorID = strings.TrimSpace(profile.SupervisorID)
	profile.ProjectIDs = normalizeOrganizationStringList(profile.ProjectIDs)
	profile.TeamRole = strings.TrimSpace(profile.TeamRole)
	profile.EmploymentStatus = normalizeOrganizationEmploymentStatus(profile.EmploymentStatus)
	if profile.PositionLevel < 0 {
		profile.PositionLevel = 0
	}
	return profile
}

func normalizeOrganizationEmploymentStatus(status string) string {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if isValidOrganizationEmploymentStatus(normalizedStatus) {
		return normalizedStatus
	}
	return organizationEmploymentStatusActive
}

func isValidOrganizationEmploymentStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case organizationEmploymentStatusActive, organizationEmploymentStatusLeave, organizationEmploymentStatusResigned:
		return true
	default:
		return false
	}
}

func organizationProfileKey(profile organizationProfile) string {
	if profile.UserID != "" {
		return "user:" + profile.UserID
	}
	if profile.Email != "" {
		return "email:" + profile.Email
	}
	return ""
}

func normalizeOrganizationStringList(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		normalizedValue := strings.TrimSpace(value)
		if normalizedValue == "" || seen[normalizedValue] {
			continue
		}
		seen[normalizedValue] = true
		result = append(result, normalizedValue)
	}
	return result
}

func normalizeOrganizationGroups(groups []orgGroupRecord) []orgGroupRecord {
	result, _ := normalizeOrganizationGroupsWithAliases(groups)
	return result
}

func normalizeOrganizationGroupsWithAliases(groups []orgGroupRecord) ([]orgGroupRecord, map[string]string) {
	result := []orgGroupRecord{}
	seenIDs := map[string]bool{}
	groupIDsByName := map[string]string{}
	aliases := map[string]string{}
	for _, group := range groups {
		id := strings.TrimSpace(group.ID)
		name := strings.TrimSpace(group.Name)
		nameKey := strings.ToLower(name)
		if id == "" || name == "" || seenIDs[id] {
			continue
		}
		seenIDs[id] = true
		if existingID := groupIDsByName[nameKey]; existingID != "" {
			aliases[id] = existingID
			continue
		}
		groupIDsByName[nameKey] = id
		result = append(result, orgGroupRecord{ID: id, Name: name, ParentID: strings.TrimSpace(group.ParentID)})
	}
	for index, group := range result {
		result[index].ParentID = canonicalOrganizationGroupID(group.ParentID, aliases)
	}
	return result, aliases
}

func encodeOrganizationStringList(values []string) (string, error) {
	document, errorValue := json.Marshal(normalizeOrganizationStringList(values))
	if errorValue != nil {
		return "", errorValue
	}
	return string(document), nil
}

func decodeOrganizationStringList(document string) []string {
	var values []string
	if json.Unmarshal([]byte(document), &values) != nil {
		return []string{}
	}
	return normalizeOrganizationStringList(values)
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
