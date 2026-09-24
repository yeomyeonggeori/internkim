package admind

import "strings"

func taskMemberIdentifier(record adminUserMutation) string {
	return stableTaskID(strings.ToLower(strings.TrimSpace(record.Email)))
}
