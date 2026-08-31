package admind

import "strings"

// The circle everyone belongs to was called staff until internkim#507. A person
// record written before that still names it, and a person in two circles that
// mean the same people is in one circle twice — with the older name pointing at
// a directory that is now empty.
func isTheCircleEveryoneIsIn(circle string) bool {
	normalized := strings.ToLower(strings.TrimSpace(circle))
	return normalized == memberCircleName || normalized == formerMemberCircleName
}
