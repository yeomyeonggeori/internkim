package admind

import "strings"

func mattermostMarkdownTableCell(value string) string {
	normalized := strings.TrimSpace(strings.NewReplacer("|", `\|`, "\n", " ", "\r", " ").Replace(value))
	if normalized == "" {
		return "-"
	}
	return normalized
}
