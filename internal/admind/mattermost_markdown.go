package admind

import "strings"

func mattermostMarkdownLink(label string, target string) string {
	return "[" + mattermostMarkdownLinkLabel(label) + "](" + target + ")"
}

func mattermostMarkdownLinkLabel(label string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(strings.TrimSpace(label))
}

func mattermostMarkdownTable(headers []string, rows [][]string) string {
	lines := []string{
		"| " + strings.Join(headers, " | ") + " |",
		"|" + strings.Repeat(" --- |", len(headers)),
	}
	for _, row := range rows {
		cells := make([]string, len(headers))
		for index := range headers {
			cellValue := ""
			if index < len(row) {
				cellValue = row[index]
			}
			cells[index] = mattermostMarkdownTableCell(cellValue)
		}
		lines = append(lines, "| "+strings.Join(cells, " | ")+" |")
	}
	return strings.Join(lines, "\n")
}

func mattermostMarkdownTableCell(value string) string {
	normalized := strings.TrimSpace(strings.NewReplacer("|", `\|`, "\n", " ", "\r", " ").Replace(value))
	if normalized == "" {
		return "-"
	}
	return normalized
}
