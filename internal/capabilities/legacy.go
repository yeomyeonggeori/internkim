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
	"site.app.create":           "site.serve",
	"site.app.delete":           "site.unserve",
	"site.app.preview":          "site.serve",
	"site.app.publish":          "site.serve",
	"site.app.status":           "site.list",
	"site.create":               "site.serve",
	"site.delete":               "site.unserve",
	"site.preview":              "site.serve",
	"site.publish":              "site.serve",
	"site.status":               "site.list",
}

func LegacyToolNameReplacements() map[string]string {
	replacements := make(map[string]string, len(legacyToolNameReplacements))
	for legacyToolName, currentToolName := range legacyToolNameReplacements {
		replacements[legacyToolName] = currentToolName
	}
	return replacements
}
