package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func blueclawPersonHasEmail(person map[string]any, email string) bool {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	emailValues, _ := person["emails"].([]any)
	for _, value := range emailValues {
		candidate, isString := value.(string)
		if isString && strings.ToLower(strings.TrimSpace(candidate)) == normalizedEmail {
			return true
		}
	}
	return false
}

func (service *Service) withBlueclawCircles(ctx context.Context, responseBody []byte) ([]byte, error) {
	var usersResponse pagesUsersResponse
	if errorValue := json.Unmarshal(responseBody, &usersResponse); errorValue != nil {
		return nil, errorValue
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	usersResponse.AvailableCircles = blueclawAvailableCircles(policyDocument)
	usersResponse.AvailableGroups = blueclawAvailableGroups(policyDocument)
	circlesByEmail := blueclawCirclesByEmail(policyDocument)
	profilesByEmail := blueclawProfilesByEmail(policyDocument)
	for index := range usersResponse.Records {
		email := strings.ToLower(strings.TrimSpace(usersResponse.Records[index].Email))
		usersResponse.Records[index].Circles = normalizeAdminUserCircles(circlesByEmail[email], usersResponse.Records[index].Role)
		profile := profilesByEmail[email]
		usersResponse.Records[index].UserID = firstNonEmpty(profile.PersonID, usersResponse.Records[index].UserID)
		if profile.Note != "" {
			usersResponse.Records[index].Note = profile.Note
		}
		usersResponse.Records[index].JobTitle = profile.JobTitle
		usersResponse.Records[index].Group = profile.Group
		usersResponse.Records[index].SupervisorID = profile.SupervisorID
	}
	return json.Marshal(usersResponse)
}

func blueclawAvailableCircles(policyDocument map[string]any) []adminCircleRecord {
	circleValues, _ := policyDocument["circles"].([]any)
	circles := []adminCircleRecord{}
	for _, value := range circleValues {
		circle, isCircle := value.(map[string]any)
		if !isCircle {
			continue
		}
		circleID := strings.ToLower(strings.TrimSpace(mattermostPolicyString(circle["circleID"])))
		if circleID == "" {
			continue
		}
		displayName := strings.TrimSpace(mattermostPolicyString(circle["displayName"]))
		if displayName == "" {
			displayName = circleID
		}
		isMattermostManaged, _ := circle["isMattermostManaged"].(bool)
		circles = append(circles, adminCircleRecord{CircleID: circleID, DisplayName: displayName, IsMattermostManaged: isMattermostManaged})
	}
	if len(circles) == 0 {
		return []adminCircleRecord{{CircleID: "staff", DisplayName: "Staff"}}
	}
	return circles
}

func blueclawCirclesByEmail(policyDocument map[string]any) map[string][]string {
	people, _ := policyDocument["people"].([]any)
	circlesByEmail := map[string][]string{}
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		circles := policyStringList(person["circles"])
		emailValues, _ := person["emails"].([]any)
		for _, emailValue := range emailValues {
			email, isString := emailValue.(string)
			if isString {
				circlesByEmail[strings.ToLower(strings.TrimSpace(email))] = circles
			}
		}
	}
	return circlesByEmail
}

type blueclawPersonProfile struct {
	PersonID     string
	Note         string
	JobTitle     string
	Group        string
	SupervisorID string
}

func blueclawAvailableGroups(policyDocument map[string]any) []orgGroupRecord {
	groupValues, _ := policyDocument["orgGroups"].([]any)
	groups := []orgGroupRecord{}
	for _, value := range groupValues {
		group, isGroup := value.(map[string]any)
		if !isGroup {
			continue
		}
		id := strings.TrimSpace(mattermostPolicyString(group["id"]))
		name := strings.TrimSpace(mattermostPolicyString(group["name"]))
		if id == "" || name == "" {
			continue
		}
		groups = append(groups, orgGroupRecord{ID: id, Name: name})
	}
	return groups
}

func blueclawProfilesByEmail(policyDocument map[string]any) map[string]blueclawPersonProfile {
	people, _ := policyDocument["people"].([]any)
	profilesByEmail := map[string]blueclawPersonProfile{}
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		profile := blueclawPersonProfile{
			PersonID:     strings.TrimSpace(mattermostPolicyString(person["personID"])),
			Note:         strings.TrimSpace(mattermostPolicyString(person["note"])),
			JobTitle:     strings.TrimSpace(mattermostPolicyString(person["jobTitle"])),
			Group:        strings.TrimSpace(mattermostPolicyString(person["group"])),
			SupervisorID: strings.TrimSpace(mattermostPolicyString(person["supervisorID"])),
		}
		emailValues, _ := person["emails"].([]any)
		for _, emailValue := range emailValues {
			email, isString := emailValue.(string)
			if isString {
				profilesByEmail[strings.ToLower(strings.TrimSpace(email))] = profile
			}
		}
	}
	return profilesByEmail
}
