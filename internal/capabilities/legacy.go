package capabilities

var legacyToolNameReplacements = map[string]string{
	"calendar.event.add":        "calendar.add",
	"calendar.event.delete":     "calendar.delete",
	"calendar.event.list":       "calendar.list",
	"calendar.event.update":     "calendar.update",
	"flow.task.add":             "task.add",
	"flow.task.delete":          "task.delete",
	"flow.task.list":            "task.list",
	"flow.task.update":          "task.update",
	"mattermost.channel.update": "channel.update",
	"platform.message.context":  "message.context",
	"platform.message.delete":   "message.delete",
	"platform.message.search":   "message.search",
	"platform.message.send":     "message.send",
	"platform.message.update":   "message.update",
	"site.app.create":           "site.create",
	"site.app.delete":           "site.delete",
	"site.app.diff":             "site.diff",
	"site.app.history":          "site.history",
	"site.app.logs":             "site.logs",
	"site.app.preview":          "site.preview",
	"site.app.publish":          "site.publish",
	"site.app.restore":          "site.restore",
	"site.app.rollback":         "site.rollback",
	"site.app.status":           "site.status",
	"site.app.unpublish":        "site.unpublish",
}

func LegacyToolNameReplacements() map[string]string {
	replacements := make(map[string]string, len(legacyToolNameReplacements))
	for legacyToolName, currentToolName := range legacyToolNameReplacements {
		replacements[legacyToolName] = currentToolName
	}
	return replacements
}
