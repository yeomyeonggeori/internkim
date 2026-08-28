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

func enrichTaskResultDocument(result json.RawMessage, members []taskMemberForTool) json.RawMessage {
	var task taskForTool
	if json.Unmarshal(result, &task) != nil || strings.TrimSpace(task.ID) == "" {
		return result
	}
	document, errorValue := json.Marshal(taskResultDocument(task, members))
	if errorValue != nil {
		return result
	}
	return document
}

func enrichTasksForTool(tasks []taskForTool, members []taskMemberForTool) []map[string]any {
	result := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, taskResultDocument(task, members))
	}
	return result
}

func taskResultDocument(task taskForTool, members []taskMemberForTool) map[string]any {
	document := map[string]any{}
	encodedTask, _ := json.Marshal(taskWithParticipantPresentations(task, members))
	json.Unmarshal(encodedTask, &document)
	delete(document, "id")
	delete(document, "createdAt")
	document["taskID"] = task.ID
	return document
}

func taskWithParticipantPresentations(task taskForTool, members []taskMemberForTool) taskForTool {
	if task.ParticipantIDs == nil {
		task.ParticipantIDs = []string{}
	}
	if task.ParticipantNames == nil {
		task.ParticipantNames = []string{}
	}
	task.ParticipantPresentations = taskParticipantPresentations(task.ParticipantIDs, task.ParticipantNames, members)
	return task
}

func taskParticipantPresentations(participantIDs []string, participantNames []string, members []taskMemberForTool) []personPresentationForTool {
	memberByID := taskMemberByID(members)
	presentations := make([]personPresentationForTool, 0, len(participantIDs))
	for index, participantID := range participantIDs {
		member, found := memberByID[strings.TrimSpace(participantID)]
		if found {
			presentations = append(presentations, personPresentationFromTaskMember(member))
			continue
		}
		presentations = append(presentations, personPresentationFromParticipantName(participantID, participantNames, index))
	}
	return presentations
}

func taskMemberByID(members []taskMemberForTool) map[string]taskMemberForTool {
	memberByID := map[string]taskMemberForTool{}
	for _, member := range members {
		memberID := strings.TrimSpace(member.ID)
		if memberID != "" {
			memberByID[memberID] = member
		}
	}
	return memberByID
}

func personPresentationFromTaskMember(member taskMemberForTool) personPresentationForTool {
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
