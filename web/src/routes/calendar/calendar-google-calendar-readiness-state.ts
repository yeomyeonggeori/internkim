import type { CalendarAccountStatusResponse, CalendarReadinessStatus } from './calendar-layout-types';
import type { CalendarGoogleAccountText } from './text';
import { isWaitingForInitialGoogleAccountStatus } from './calendar-google-account-state';

export function googleCalendarConnectionStateLabel(
	text: CalendarGoogleAccountText,
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): string {
	if (
		isWaitingForInitialGoogleAccountStatus(isLoadingAccountStatus, accountStatus) ||
		accountStatusError ||
		!accountStatus?.connected
	) {
		return '';
	}
	switch (googleCalendarReadinessStatus(accountStatus)) {
		case 'reauth_required':
			return text.googleCalendarReauthRequired;
		case 'calendar_selection_required':
			return text.googleCalendarSelectionRequired;
		case 'write_permission_required':
			return text.googleCalendarWritePermissionRequired;
		case 'calendar_inaccessible':
			return text.googleCalendarInaccessible;
		case 'sync_ready':
			return text.googleCalendarSyncReady;
		case 'initial_export_pending':
			return text.googleCalendarInitialExportPending;
		case 'initial_sync_pending':
		default:
			return text.googleCalendarInitialSyncPending;
	}
}

export function googleCalendarConnectionStateClass(accountStatus: CalendarAccountStatusResponse | null): string {
	if (!accountStatus?.connected) {
		return 'border-border bg-muted text-muted-foreground';
	}
	const readinessStatus = googleCalendarReadinessStatus(accountStatus);
	if (readinessStatus === 'sync_ready') {
		return 'border-emerald-200 bg-emerald-50 text-emerald-700';
	}
	if (readinessStatus === 'initial_export_pending') {
		return 'border-amber-200 bg-amber-50 text-amber-700';
	}
	if (
		readinessStatus === 'reauth_required' ||
		readinessStatus === 'write_permission_required' ||
		readinessStatus === 'calendar_inaccessible'
	) {
		return 'border-destructive/30 bg-destructive/10 text-destructive';
	}
	return 'border-border bg-muted text-muted-foreground';
}

export function googleCalendarReadinessStatus(status: CalendarAccountStatusResponse): CalendarReadinessStatus {
	if (status.calendarReadinessStatus) return status.calendarReadinessStatus;
	if (status.needsReauth) return 'reauth_required';
	if (status.needsCalendarSelection) return 'calendar_selection_required';
	if (!status.initialSyncCompleted) return 'initial_sync_pending';
	if (status.calendarSyncReady) return 'sync_ready';
	return 'write_permission_required';
}
