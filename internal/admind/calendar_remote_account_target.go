package admind

import "strings"

type remoteCalendarTarget struct {
	CalendarURL                string
	CalendarCTag               string
	IsSelectedCalendar         bool
	NeedsInitialSyncCompletion bool
}

func activeRemoteCalendarTarget(account remoteCalendarAccount) remoteCalendarTarget {
	if target := selectedRemoteCalendarTarget(account); target.CalendarURL != "" {
		return target
	}
	selectedCalendarID := strings.TrimSpace(account.SelectedCalendarID)
	selectedCalendarURL := strings.TrimSpace(account.SelectedCalendarURL)
	if selectedCalendarID != "" || selectedCalendarURL != "" {
		return remoteCalendarTarget{}
	}
	return remoteCalendarTarget{
		CalendarURL:  strings.TrimSpace(account.DefaultCalendarURL),
		CalendarCTag: strings.TrimSpace(account.DefaultCalendarCTag),
	}
}

func selectedRemoteCalendarTarget(account remoteCalendarAccount) remoteCalendarTarget {
	selectedCalendarID := strings.TrimSpace(account.SelectedCalendarID)
	selectedCalendarURL := strings.TrimSpace(account.SelectedCalendarURL)
	if selectedCalendarID != "" && selectedCalendarURL != "" {
		return remoteCalendarTarget{
			CalendarURL:                selectedCalendarURL,
			CalendarCTag:               strings.TrimSpace(account.DefaultCalendarCTag),
			IsSelectedCalendar:         true,
			NeedsInitialSyncCompletion: strings.TrimSpace(account.InitialSyncCompletedAt) == "",
		}
	}
	return remoteCalendarTarget{}
}

func remoteCalendarEventBelongsToTarget(event calendarEvent, target remoteCalendarTarget) bool {
	return remoteCalendarHrefBelongsToTarget(event.RemoteHref, target)
}

func remoteCalendarHrefBelongsToTarget(remoteHref string, target remoteCalendarTarget) bool {
	if !target.IsSelectedCalendar {
		return true
	}
	calendarURL := strings.TrimRight(calDAVPathOnly(strings.TrimSpace(target.CalendarURL)), "/")
	if calendarURL == "" {
		return true
	}
	normalizedRemoteHref := calDAVPathOnly(strings.TrimSpace(remoteHref))
	if normalizedRemoteHref == "" {
		return false
	}
	return strings.HasPrefix(normalizedRemoteHref, calendarURL+"/")
}
