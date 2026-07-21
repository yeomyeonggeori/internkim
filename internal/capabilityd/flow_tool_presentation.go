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
	var task flowTaskForTool
	if json.Unmarshal(result, &task) != nil || strings.TrimSpace(task.ID) == "" {
		return result
	}
	document, errorValue := json.Marshal(flowTaskResultDocument(task, members))
	if errorValue != nil {
		return result
	}
	return document
}

func enrichFlowTasksForTool(tasks []flowTaskForTool, members []flowMemberForTool) []map[string]any {
	result := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, flowTaskResultDocument(task, members))
	}
	return result
}

func flowTaskResultDocument(task flowTaskForTool, members []flowMemberForTool) map[string]any {
	document := map[string]any{}
	encodedTask, _ := json.Marshal(flowTaskWithParticipantPresentations(task, members))
	json.Unmarshal(encodedTask, &document)
	delete(document, "id")
	delete(document, "createdAt")
	document["taskID"] = task.ID
	return document
}

func flowTaskWithParticipantPresentations(task flowTaskForTool, members []flowMemberForTool) flowTaskForTool {
	if task.ParticipantIDs == nil {
		task.ParticipantIDs = []string{}
	}
	if task.ParticipantNames == nil {
		task.ParticipantNames = []string{}
	}
	task.ParticipantPresentations = flowParticipantPresentations(task.ParticipantIDs, task.ParticipantNames, members)
	return task
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
