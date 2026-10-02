package capabilities

var legacyToolNameReplacements = map[string]string{
	"calendar_add":             "event_add",
	"calendar_delete":          "event_delete",
	"calendar_list":            "event_list",
	"calendar_update":          "event_update",
	"calendar.event.add":       "event_add",
	"calendar.event.delete":    "event_delete",
	"calendar.event.list":      "event_list",
	"calendar.event.update":    "event_update",
	"flow.task.add":            "task_add",
	"flow.task.delete":         "task_delete",
	"flow.task.list":           "task_list",
	"flow.task.update":         "task_update",
	"platform.message.context": "message_context",
	"platform.message.delete":  "message_delete",
	"platform.message.search":  "message_search",
	"platform.message.send":    "message_send",
	"platform.message.update":  "message_update",
}

func LegacyToolNameReplacements() map[string]string {
	replacements := make(map[string]string, len(legacyToolNameReplacements))
	for legacyToolName, currentToolName := range legacyToolNameReplacements {
		replacements[legacyToolName] = currentToolName
	}
	return replacements
}
