package admind

import (
	"encoding/json"
	"strings"
)

func normalizeOrgchartProfile(profile orgchartProfile) orgchartProfile {
	profile.UserID = strings.TrimSpace(profile.UserID)
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.JobTitle = strings.TrimSpace(profile.JobTitle)
	profile.PrimaryGroupID = strings.TrimSpace(profile.PrimaryGroupID)
	profile.GroupIDs = orgchartGroupIDsWithPrimary(profile.GroupIDs, profile.PrimaryGroupID)
	profile.SupervisorID = strings.TrimSpace(profile.SupervisorID)
	profile.ProjectIDs = normalizeOrgchartStringList(profile.ProjectIDs)
	profile.TeamRole = strings.TrimSpace(profile.TeamRole)
	profile.EmploymentStatus = normalizeOrgchartEmploymentStatus(profile.EmploymentStatus)
	if profile.PositionLevel < 0 {
		profile.PositionLevel = 0
	}
	return profile
}

func normalizeOrgchartEmploymentStatus(status string) string {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if isValidOrgchartEmploymentStatus(normalizedStatus) {
		return normalizedStatus
	}
	return orgchartEmploymentStatusActive
}

func isValidOrgchartEmploymentStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case orgchartEmploymentStatusActive, orgchartEmploymentStatusLeave, orgchartEmploymentStatusResigned:
		return true
	default:
		return false
	}
}

func orgchartProfileKey(profile orgchartProfile) string {
	if profile.UserID != "" {
		return "user:" + profile.UserID
	}
	if profile.Email != "" {
		return "email:" + profile.Email
	}
	return ""
}

func normalizeOrgchartStringList(values []string) []string {
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

func orgchartGroupIDsWithPrimary(groupIDs []string, primaryGroupID string) []string {
	normalizedGroupIDs := normalizeOrgchartStringList(groupIDs)
	normalizedPrimaryGroupID := strings.TrimSpace(primaryGroupID)
	result := []string{}
	seenGroupIDs := map[string]bool{}
	appendGroupID := func(groupID string) {
		if groupID == "" || seenGroupIDs[groupID] {
			return
		}
		seenGroupIDs[groupID] = true
		result = append(result, groupID)
	}
	appendGroupID(normalizedPrimaryGroupID)
	for _, groupID := range normalizedGroupIDs {
		appendGroupID(groupID)
	}
	return result
}

func normalizeOrgchartGroups(groups []orgGroupRecord) []orgGroupRecord {
	result, _ := normalizeOrgchartGroupsWithAliases(groups)
	return result
}

func normalizeOrgchartGroupsWithAliases(groups []orgGroupRecord) ([]orgGroupRecord, map[string]string) {
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
		result = append(result, orgGroupRecord{ID: id, Name: name})
	}
	return result, aliases
}

func encodeOrgchartStringList(values []string) (string, error) {
	document, errorValue := json.Marshal(normalizeOrgchartStringList(values))
	if errorValue != nil {
		return "", errorValue
	}
	return string(document), nil
}

func decodeOrgchartStringList(document string) []string {
	var values []string
	if json.Unmarshal([]byte(document), &values) != nil {
		return []string{}
	}
	return normalizeOrgchartStringList(values)
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
