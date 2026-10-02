package blueclaw

type defaultCircleDefinition struct {
	CircleID    string
	DisplayName string
}

var defaultCircleDefinitions = []defaultCircleDefinition{
	{CircleID: "member", DisplayName: "Member"},
	{CircleID: "c-level", DisplayName: "C-level"},
	{CircleID: "representative", DisplayName: "Representative"},
	{CircleID: "admin", DisplayName: "Admin"},
	{CircleID: "hr", DisplayName: "HR"},
}

func defaultResourceAccessPolicies() []map[string]any {
	policies := []map[string]any{}
	for _, circleDefinition := range defaultCircleDefinitions {
		actions := []string{"read", "write"}
		if circleDefinition.CircleID == "admin" {
			actions = append(actions, "manage")
		}
		policies = append(policies, map[string]any{
			"resource": "file:circle:" + circleDefinition.CircleID,
			"actions":  actions,
			"circles":  []string{circleDefinition.CircleID},
		})
	}
	return append(policies, []map[string]any{
		{"resource": "api:flow.summary", "actions": []string{"read"}, "circles": []string{"member"}},
		{"resource": "api:flow.task", "actions": []string{"create", "update"}, "circles": []string{"member"}},
		{"resource": "api:flow.definition", "actions": []string{"manage"}, "circles": []string{"admin"}},
		{"resource": "api:credentials.providers", "actions": []string{"manage"}, "circles": []string{"admin"}},
		{"resource": "tool:web_search", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:web_fetch", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:artifact_review", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:task_add", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:task_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:task_update", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_add", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_update", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:attendance_delete", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_context", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_search", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_send", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_update", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:message_delete", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_list", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_search", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_read", "actions": []string{"execute"}, "circles": []string{"member"}},
		{"resource": "tool:mail_message_send", "actions": []string{"execute"}, "circles": []string{"member"}},
	}...)
}
