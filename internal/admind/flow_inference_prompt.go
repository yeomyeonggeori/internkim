package admind

import (
	"fmt"
	"strings"
	"time"
)

func flowLLMRequest(prompt string, weekCode string, owner flowMember, members []flowMember, definitions flowDefinitions) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You convert short task notes into a weekly work tracker task. Return only values allowed by the schema. Keep Korean task content concise. Pick the closest type and size. Use status 예정 unless the note clearly says 진행, 완료, 요청, 기각, 일시정지, or 중단. Use YYYY-MM-DD dates only when the note clearly names a date; otherwise use empty strings.",
			},
			{
				"role":    "user",
				"content": flowInferencePrompt(prompt, weekCode, owner, members, definitions),
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "flow_task_inference",
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
					"size":           map[string]any{"type": "string", "enum": flowSizeNames(definitions.Sizes)},
					"status":         map[string]any{"type": "string", "enum": flowStatusOptions()},
					"startDate":      map[string]any{"type": "string"},
					"endDate":        map[string]any{"type": "string"},
					"participantIDs": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": memberIDOptions(members)}},
					"requestReason":  map[string]any{"type": "string"},
				},
			},
		},
	}
}

func flowInferencePrompt(prompt string, weekCode string, owner flowMember, members []flowMember, definitions flowDefinitions) string {
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
		"Size rubric: " + flowSizeRubricForPrompt(definitions.Sizes),
	}, "\n")
}

func flowSizeRubricForPrompt(sizes []flowSizeDefinition) string {
	lines := make([]string, 0, len(sizes))
	for _, size := range sizes {
		lines = append(lines, fmt.Sprintf("%s=%dkm max %dh; dev: %s; other: %s; note: %s", size.Name, size.DistanceKM, size.MaxHours, size.DevelopmentExample, size.OtherExample, size.Note))
	}
	return strings.Join(lines, " | ")
}

func memberIDOptions(members []flowMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

func flowSizeNames(sizes []flowSizeDefinition) []string {
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
