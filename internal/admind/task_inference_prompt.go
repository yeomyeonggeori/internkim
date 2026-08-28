package admind

import (
	"fmt"
	"strings"
	"time"
)

func taskLLMRequest(prompt string, weekCode string, owner taskMember, members []taskMember, definitions taskDefinitions) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You convert short task notes into a weekly work tracker task. Return only values allowed by the schema. Keep Korean task content concise. Pick the closest type and size. Use status planned unless the note clearly says the work is in_progress, completed, requested, rejected, paused, or cancelled. Use YYYY-MM-DD dates only when the note clearly names a date; otherwise use empty strings. participantIDs: the IDs from Members of the people the note assigns the work to, in the note's order. Leave it empty when the note names nobody — never add a member the note does not name. 전체, 모두, or everyone means every member.",
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
				"required":             []string{"category", "type", "content", "goal", "size", "status", "startDate", "endDate", "participantIDs", "requestReason"},
				"properties": map[string]any{
					"category":       map[string]any{"type": "string", "enum": append([]string{""}, definitions.Categories...)},
					"type":           map[string]any{"type": "string", "enum": definitions.Types},
					"content":        map[string]any{"type": "string"},
					"goal":           map[string]any{"type": "string"},
					"size":           map[string]any{"type": "string", "enum": taskSizeNames(definitions.Sizes)},
					"status":         map[string]any{"type": "string", "enum": taskStatusOptions()},
					"startDate":      map[string]any{"type": "string"},
					"endDate":        map[string]any{"type": "string"},
					"participantIDs": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": memberIDOptions(members)}},
					"requestReason":  map[string]any{"type": "string"},
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
		"Categories: " + strings.Join(definitions.Categories, ", "),
		"Types: " + strings.Join(definitions.Types, ", "),
		"Size rubric: " + taskSizeRubricForPrompt(definitions.Sizes),
	}, "\n")
}

func taskSizeRubricForPrompt(sizes []taskSizeDefinition) string {
	lines := make([]string, 0, len(sizes))
	for _, size := range sizes {
		lines = append(lines, fmt.Sprintf("%s=%dkm max %dh; dev: %s; other: %s; note: %s", size.Name, size.DistanceKM, size.MaxHours, size.DevelopmentExample, size.OtherExample, size.Note))
	}
	return strings.Join(lines, " | ")
}

func memberIDOptions(members []taskMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

func taskSizeNames(sizes []taskSizeDefinition) []string {
	values := make([]string, 0, len(sizes))
	for _, size := range sizes {
		if strings.TrimSpace(size.Name) != "" {
			values = append(values, size.Name)
		}
	}
	if len(values) == 0 {
		return []string{"XS", "S", "M", "L", "XL", "XXL"}
	}
	return values
}
