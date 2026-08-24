package admind

import (
	"net/url"
	"regexp"
	"strings"
)

// A link written before the company had an address names a device host and
// carries an identifier only that device knows. Both have to change together:
// the address alone points the reader at a record that has never heard of the
// identifier beside it.
var oldLinkPattern = regexp.MustCompile(`https?://[^\s)"'<>]+/(calendar|flow)/\?[^\s)"'<>]*`)

type linkTranslation struct {
	appURL           string
	deviceHost       string
	recordCalendarID func(string) string
	recordTaskID     func(string) string
}

func (translation linkTranslation) rewrite(content string) string {
	return oldLinkPattern.ReplaceAllStringFunc(content, func(link string) string {
		parsed, errorValue := url.Parse(link)
		if errorValue != nil || !strings.EqualFold(parsed.Host, translation.deviceHost) {
			return link
		}
		identifier, resolve := "event", translation.recordCalendarID
		if strings.HasPrefix(parsed.Path, "/flow/") {
			identifier, resolve = "task", translation.recordTaskID
		}
		query := parsed.Query()
		if given := strings.TrimSpace(query.Get(identifier)); given != "" {
			if resolved := resolve(given); resolved != "" {
				query.Set(identifier, resolved)
			} else {
				query.Del(identifier)
			}
		}
		rewritten := strings.TrimRight(translation.appURL, "/") + parsed.Path
		if encoded := query.Encode(); encoded != "" {
			rewritten += "?" + encoded
		}
		return rewritten
	})
}
