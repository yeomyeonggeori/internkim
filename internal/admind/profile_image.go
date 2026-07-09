package admind

import (
	"encoding/json"
	"strings"
)

func profileImagePathForEmail(email string) string {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return ""
	}
	return calendarParticipantImagePath(stableFlowID(normalizedEmail))
}

func adminUserRecordsWithProfileImages(records []adminUserMutation) []adminUserMutation {
	result := append([]adminUserMutation(nil), records...)
	for index := range result {
		if strings.TrimSpace(result[index].Image) != "" {
			continue
		}
		result[index].Image = profileImagePathForEmail(result[index].Email)
	}
	return result
}

func usersResponseBodyWithProfileImages(responseBody []byte) []byte {
	var document map[string]json.RawMessage
	if json.Unmarshal(responseBody, &document) != nil {
		return responseBody
	}
	recordsDocument, found := document["records"]
	if !found {
		return responseBody
	}
	var records []adminUserMutation
	if json.Unmarshal(recordsDocument, &records) != nil {
		return responseBody
	}
	updatedRecords, errorValue := json.Marshal(adminUserRecordsWithProfileImages(records))
	if errorValue != nil {
		return responseBody
	}
	document["records"] = updatedRecords
	updatedBody, errorValue := json.Marshal(document)
	if errorValue != nil {
		return responseBody
	}
	return updatedBody
}
