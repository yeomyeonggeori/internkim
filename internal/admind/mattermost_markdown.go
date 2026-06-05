package admind

import "strings"

func mattermostMarkdownLink(label string, target string) string {
	return "[" + mattermostMarkdownLinkLabel(label) + "](" + target + ")"
}

func mattermostMarkdownLinkLabel(label string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(strings.TrimSpace(label))
}
