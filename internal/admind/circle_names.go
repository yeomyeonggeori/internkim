package admind

import "strings"

const (
	memberCircleName       = "member"
	formerMemberCircleName = "staff"
)

func isTheCircleEveryoneIsIn(circle string) bool {
	normalized := strings.ToLower(strings.TrimSpace(circle))
	return normalized == memberCircleName || normalized == formerMemberCircleName
}
