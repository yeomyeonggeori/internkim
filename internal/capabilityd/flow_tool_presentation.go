package capabilityd

import (
	"encoding/json"
	"strings"
)

type personPresentationForTool struct {
	PersonID           string `json:"personID,omitempty"`
	DisplayName        string `json:"displayName,omitempty"`
	Email              string `json:"email,omitempty"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	MattermostMention  string `json:"mention,omitempty"`
}

func enrichFlowTaskResultDocument(result json.RawMessage, members []flowMemberForTool) json.RawMessage {
	object := map[string]any{}
	if json.Unmarshal(result, &object) != nil {
		return result
	}
	if _, hasTaskID := object["id"]; hasTaskID {
		enrichFlowTaskMap(object, members)
	}
	if duplicateTask, ok := object["duplicateTask"].(map[string]any); ok {
		enrichFlowTaskMap(duplicateTask, members)
		object["duplicateTask"] = duplicateTask
	}
	document, errorValue := json.Marshal(object)
	if errorValue != nil {
		return result
	}
	return document
}

func enrichFlowTasksForTool(tasks []flowTaskForTool, members []flowMemberForTool) []flowTaskForTool {
	result := append([]flowTaskForTool(nil), tasks...)
	for index := range result {
		result[index] = flowTaskWithParticipantPresentations(result[index], members)
	}
	return result
}

func flowTaskWithParticipantPresentations(task flowTaskForTool, members []flowMemberForTool) flowTaskForTool {
	task.ParticipantPresentations = flowParticipantPresentations(task.ParticipantIDs, task.ParticipantNames, members)
	return task
}

func enrichFlowTaskMap(task map[string]any, members []flowMemberForTool) {
	participantIDs := stringValuesFromUnknown(task["participantIDs"])
	participantNames := stringValuesFromUnknown(task["participantNames"])
	presentations := flowParticipantPresentations(participantIDs, participantNames, members)
	task["participantPresentations"] = presentations
}

func flowParticipantPresentations(participantIDs []string, participantNames []string, members []flowMemberForTool) []personPresentationForTool {
	memberByID := flowMemberByID(members)
	presentations := make([]personPresentationForTool, 0, len(participantIDs))
	for index, participantID := range participantIDs {
		member, found := memberByID[strings.TrimSpace(participantID)]
		if found {
			presentations = append(presentations, personPresentationFromFlowMember(member))
			continue
		}
		presentations = append(presentations, personPresentationFromParticipantName(participantID, participantNames, index))
	}
	return presentations
}

func flowMemberByID(members []flowMemberForTool) map[string]flowMemberForTool {
	memberByID := map[string]flowMemberForTool{}
	for _, member := range members {
		memberID := strings.TrimSpace(member.ID)
		if memberID != "" {
			memberByID[memberID] = member
		}
	}
	return memberByID
}

func personPresentationFromFlowMember(member flowMemberForTool) personPresentationForTool {
	return personPresentationForTool{
		PersonID:           strings.TrimSpace(member.ID),
		DisplayName:        strings.TrimSpace(member.Name),
		Email:              strings.ToLower(strings.TrimSpace(member.Email)),
		MattermostUsername: strings.TrimSpace(member.MattermostUsername),
		MattermostMention:  mattermostMentionForUsername(member.MattermostUsername),
	}
}

func personPresentationFromParticipantName(participantID string, participantNames []string, index int) personPresentationForTool {
	displayName := ""
	if index >= 0 && index < len(participantNames) {
		displayName = strings.TrimSpace(participantNames[index])
	}
	return personPresentationForTool{
		PersonID:    strings.TrimSpace(participantID),
		DisplayName: displayName,
	}
}

func stringValuesFromUnknown(value any) []string {
	switch typedValue := value.(type) {
	case []string:
		return uniqueTrimmedStringValues(typedValue)
	case []any:
		values := make([]string, 0, len(typedValue))
		for _, item := range typedValue {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return uniqueTrimmedStringValues(values)
	default:
		return nil
	}
}

func uniqueTrimmedStringValues(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || seen[trimmedValue] {
			continue
		}
		seen[trimmedValue] = true
		result = append(result, trimmedValue)
	}
	return result
}
