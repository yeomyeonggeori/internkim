package admind

import (
	"encoding/json"
	"strings"
)

func normalizeOrganizationProfile(profile organizationProfile) organizationProfile {
	profile.MemberID = strings.TrimSpace(profile.MemberID)
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.JobTitle = strings.TrimSpace(profile.JobTitle)
	profile.GroupID = strings.TrimSpace(profile.GroupID)
	profile.HireDate = strings.TrimSpace(profile.HireDate)
	profile.SupervisorID = strings.TrimSpace(profile.SupervisorID)
	profile.Status = normalizeMemberStatus(profile.Status)
	return profile
}

func normalizeMemberStatus(status string) string {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	if normalizedStatus == "" {
		return memberStatusActive
	}
	return normalizedStatus
}

func isMemberStillHere(status string) bool {
	switch normalizeMemberStatus(status) {
	case memberStatusDeparted, memberStatusWithdrawn:
		return false
	default:
		return true
	}
}

func organizationProfileKey(profile organizationProfile) string {
	if profile.MemberID != "" {
		return "user:" + profile.MemberID
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
