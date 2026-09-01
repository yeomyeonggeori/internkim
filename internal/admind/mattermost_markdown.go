package admind

import "strings"


func mattermostMarkdownLinkLabel(label string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(strings.TrimSpace(label))
}


func mattermostMarkdownTableCell(value string) string {
	normalized := strings.TrimSpace(strings.NewReplacer("|", `\|`, "\n", " ", "\r", " ").Replace(value))
	if normalized == "" {
		return "-"
	}
	return normalized
}
