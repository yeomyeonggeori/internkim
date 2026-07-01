package admind

import "strings"

func isWritableRemoteCalendarAccessRole(accessRole string) bool {
	normalizedAccessRole := strings.ToLower(strings.TrimSpace(accessRole))
	return normalizedAccessRole == "writer" || normalizedAccessRole == "owner"
}

func remoteCalendarAccountCanWrite(account remoteCalendarAccount) bool {
	target := activeRemoteCalendarTarget(account)
	if !target.IsSelectedCalendar {
		return true
	}
	return isWritableRemoteCalendarAccessRole(account.SelectedCalendarAccessRole)
}
