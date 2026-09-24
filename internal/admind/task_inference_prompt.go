package admind

import (
	"strings"
	"time"
)

func taskLLMRequest(prompt string, weekCode string, owner taskMember, members []taskMember, definitions taskDefinitions) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You convert short task notes into a weekly work tracker task. Return only values allowed by the schema. Keep Korean task content concise. Use status planned unless the note clearly says the work is in_progress, completed, requested, rejected, paused, or cancelled. Use YYYY-MM-DD dates only when the note clearly names a date; otherwise use empty strings. participantIDs: the IDs from Members of the people the note assigns the work to, in the note's order. Leave it empty when the note names nobody — never add a member the note does not name. 전체, 모두, or everyone means every member.",
			},
			{
				"role":    "user",
				"content": taskInferencePrompt(prompt, weekCode, owner, members, definitions),
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "task_inference",
			"isStrictlyEnforced": true,
			"document": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"content", "goal", "status", "startDate", "endDate", "participantIDs"},
				"properties": map[string]any{
					"content":        map[string]any{"type": "string"},
					"goal":           map[string]any{"type": "string"},
					"status":         map[string]any{"type": "string", "enum": taskStatusOptions()},
					"startDate":      map[string]any{"type": "string"},
					"endDate":        map[string]any{"type": "string"},
					"participantIDs": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": memberIDOptions(members)}},
				},
			},
		},
	}
}

func taskInferencePrompt(prompt string, weekCode string, owner taskMember, members []taskMember, definitions taskDefinitions) string {
	memberLines := make([]string, 0, len(members))
	for _, member := range members {
		memberLines = append(memberLines, member.ID+"="+member.Name+"<"+member.Email+">")
	}
	now := time.Now()
	resolvedWeekCode := firstNonEmpty(strings.TrimSpace(weekCode), weekCodeForDate(now))
	weekStart := weekStartForCode(resolvedWeekCode, now)
	return strings.Join([]string{
		"Task note: " + prompt,
		"Today: " + now.Format("2006-01-02"),
		"Week code: " + resolvedWeekCode,
		"Week dates: " + weekStart.Format("2006-01-02") + " to " + weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		"Default owner ID: " + owner.ID,
		"Members: " + strings.Join(memberLines, ", "),
	}, "\n")
}


func memberIDOptions(members []taskMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

