package admind

import "strings"

const (
	calendarReadinessStatusCalendarSelectionRequired = "calendar_selection_required"
	calendarReadinessStatusInitialSyncPending        = "initial_sync_pending"
	calendarReadinessStatusInitialExportPending      = "initial_export_pending"
	calendarReadinessStatusSyncReady                 = "sync_ready"
	calendarReadinessStatusWritePermissionRequired   = "write_permission_required"
	calendarReadinessStatusCalendarInaccessible      = "calendar_inaccessible"
	calendarReadinessStatusReauthRequired            = "reauth_required"
)

func calendarReadinessStatusForAccount(account remoteCalendarAccount) string {
	if strings.TrimSpace(account.LastAuthError) != "" {
		return calendarReadinessStatusReauthRequired
	}
	if strings.TrimSpace(account.SelectedCalendarID) == "" || strings.TrimSpace(account.SelectedCalendarURL) == "" {
		return calendarReadinessStatusCalendarSelectionRequired
	}
	if !remoteCalendarAccountCanWrite(account) {
		return calendarReadinessStatusWritePermissionRequired
	}
	selectedCalendarReadinessStatus := strings.TrimSpace(account.SelectedCalendarReadinessStatus)
	if selectedCalendarReadinessStatus == calendarReadinessStatusCalendarInaccessible {
		return calendarReadinessStatusCalendarInaccessible
	}
	if selectedCalendarReadinessStatus == calendarReadinessStatusInitialExportPending {
		return calendarReadinessStatusInitialExportPending
	}
	if strings.TrimSpace(account.InitialSyncCompletedAt) == "" {
		return calendarReadinessStatusInitialSyncPending
	}
	return calendarReadinessStatusSyncReady
}
