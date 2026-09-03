package admind

import (
	"encoding/json"
	"net/http"
	"net/url"

	"strings"
)

func (service *Service) saveBlueclawCircle(responseWriter http.ResponseWriter, request *http.Request) {
	var input adminCircleRecord
	if errorValue := json.NewDecoder(request.Body).Decode(&input); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	circleID := strings.ToLower(strings.TrimSpace(input.CircleID))
	if circleID == "" {
		http.Error(responseWriter, "circleID required", http.StatusBadRequest)
		return
	}
	if isReservedAdminCircleID(circleID) {
		http.Error(responseWriter, "reserved group cannot be changed", http.StatusBadRequest)
		return
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	policyDocument["circles"] = upsertBlueclawCircle(policyDocument["circles"], adminCircleRecord{
		CircleID:            circleID,
		DisplayName:         firstNonEmpty(strings.TrimSpace(input.DisplayName), circleID),
		IsMattermostManaged: input.IsMattermostManaged,
	})
	policyDocument["circleSync"] = upsertBlueclawCircleSync(policyDocument["circleSync"], circleID, input.IsMattermostManaged)
	if errorValue := service.deliverBlueclawPolicy(request.Context(), policyDocument); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"availableCircles": blueclawAvailableCircles(policyDocument)})
}

func (service *Service) deleteBlueclawCircle(responseWriter http.ResponseWriter, request *http.Request, encodedCircleID string) {
	circleID, _ := url.PathUnescape(encodedCircleID)
	circleID = strings.ToLower(strings.TrimSpace(circleID))
	if circleID == "" {
		http.Error(responseWriter, "circleID required", http.StatusBadRequest)
		return
	}
	if isReservedAdminCircleID(circleID) {
		http.Error(responseWriter, "reserved group cannot be removed", http.StatusBadRequest)
		return
	}
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	policyDocument["circles"] = removeBlueclawCircle(policyDocument["circles"], circleID)
	policyDocument["circleSync"] = removeBlueclawCircleSync(policyDocument["circleSync"], circleID)
	removeCircleFromBlueclawPeople(policyDocument["people"], circleID)
	if errorValue := service.deliverBlueclawPolicy(request.Context(), policyDocument); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]any{"availableCircles": blueclawAvailableCircles(policyDocument)})
}

func normalizeAdminUserCircles(circles []string, role string) []string {
	normalizedCircles := []string{"member"}
	if normalizeAdminUserRole(role) == "admin" {
		normalizedCircles = append(normalizedCircles, "admin")
	}
	for _, circle := range circles {
		normalizedCircle := strings.ToLower(strings.TrimSpace(circle))
		if normalizedCircle == "" || isTheCircleEveryoneIsIn(normalizedCircle) {
			continue
		}
		normalizedCircles = append(normalizedCircles, normalizedCircle)
	}
	return uniqueAdminStrings(normalizedCircles)
}

func uniqueAdminStrings(values []string) []string {
	seenValue := map[string]bool{}
	result := []string{}
	for _, value := range values {
		normalizedValue := strings.ToLower(strings.TrimSpace(value))
		if normalizedValue == "" || seenValue[normalizedValue] {
			continue
		}
		seenValue[normalizedValue] = true
		result = append(result, normalizedValue)
	}
	return result
}

func upsertBlueclawCircle(value any, circle adminCircleRecord) []any {
	circleValues, _ := value.([]any)
	result := make([]any, 0, len(circleValues)+1)
	isUpdated := false
	for _, item := range circleValues {
		existingCircle, isCircle := item.(map[string]any)
		if !isCircle {
			continue
		}
		if strings.ToLower(strings.TrimSpace(policyString(existingCircle["circleID"]))) == circle.CircleID {
			existingCircle["displayName"] = circle.DisplayName
			existingCircle["isMattermostManaged"] = circle.IsMattermostManaged
			existingCircle["workspaceDirectoryPath"] = "/workspace/circles/" + circle.CircleID
			isUpdated = true
		}
		result = append(result, existingCircle)
	}
	if !isUpdated {
		result = append(result, map[string]any{
			"circleID":               circle.CircleID,
			"displayName":            circle.DisplayName,
			"isMattermostManaged":    circle.IsMattermostManaged,
			"workspaceDirectoryPath": "/workspace/circles/" + circle.CircleID,
		})
	}
	return result
}

func removeBlueclawCircle(value any, circleID string) []any {
	circleValues, _ := value.([]any)
	result := []any{}
	for _, item := range circleValues {
		circle, isCircle := item.(map[string]any)
		if !isCircle || strings.ToLower(strings.TrimSpace(policyString(circle["circleID"]))) == circleID {
			continue
		}
		result = append(result, circle)
	}
	return result
}

func upsertBlueclawCircleSync(value any, circleID string, isMattermostManaged bool) map[string]any {
	circleSync, _ := value.(map[string]any)
	if circleSync == nil {
		circleSync = map[string]any{}
	}
	if !isMattermostManaged {
		circleSync["mattermostPrivateChannels"] = removeMattermostCircleSync(circleSync["mattermostPrivateChannels"], circleID)
		return circleSync
	}
	channelValues := removeMattermostCircleSync(circleSync["mattermostPrivateChannels"], circleID)
	channelValues = append(channelValues, map[string]any{"circleID": circleID, "channelName": "circle-" + circleID})
	circleSync["mattermostPrivateChannels"] = channelValues
	return circleSync
}

func removeBlueclawCircleSync(value any, circleID string) map[string]any {
	circleSync, _ := value.(map[string]any)
	if circleSync == nil {
		return map[string]any{}
	}
	circleSync["mattermostPrivateChannels"] = removeMattermostCircleSync(circleSync["mattermostPrivateChannels"], circleID)
	return circleSync
}

func removeMattermostCircleSync(value any, circleID string) []any {
	channelValues, _ := value.([]any)
	result := []any{}
	for _, item := range channelValues {
		channel, isChannel := item.(map[string]any)
		if !isChannel || strings.ToLower(strings.TrimSpace(policyString(channel["circleID"]))) == circleID {
			continue
		}
		result = append(result, channel)
	}
	return result
}

func removeCircleFromBlueclawPeople(value any, circleID string) {
	people, _ := value.([]any)
	for _, item := range people {
		person, isPerson := item.(map[string]any)
		if !isPerson {
			continue
		}
		person["circles"] = removeAdminString(policyStringList(person["circles"]), circleID)
	}
}

func removeAdminString(values []string, removedValue string) []string {
	result := []string{}
	for _, value := range values {
		if strings.ToLower(strings.TrimSpace(value)) != removedValue {
			result = append(result, value)
		}
	}
	return result
}
