export type CalendarSyncResponse = {
	caldavURL: string;
	caldavUsername: string;
	caldavPassword: string;
	icsURL: string;
};

export type CalendarAccountStatusResponse = {
	connected: boolean;
	provider?: string;
	accountEmail?: string;
	selectedCalendarID?: string;
	selectedCalendarName?: string;
	selectedCalendarAccessRole?: string;
	lastAuthError?: string;
	needsReauth: boolean;
	needsCalendarSelection: boolean;
	initialSyncCompleted: boolean;
	calendarSyncReady: boolean;
	calendarReadinessStatus?: CalendarReadinessStatus;
	googleOAuthConfigured: boolean;
	canManageGoogleOAuth: boolean;
};

export type CalendarReadinessStatus =
	| 'calendar_selection_required'
	| 'initial_sync_pending'
	| 'initial_export_pending'
	| 'sync_ready'
	| 'write_permission_required'
	| 'calendar_inaccessible'
	| 'reauth_required';

export type GoogleCalendarListEntry = {
	calendarID: string;
	summary: string;
	accessRole: string;
	timeZone?: string;
	primary: boolean;
	backgroundColor?: string;
	canWrite: boolean;
	canSelect: boolean;
	selectionDisabledReason?: GoogleCalendarSelectionDisabledReason;
};

export type GoogleCalendarSelectionDisabledReason = 'write_permission_required' | 'unsupported_calendar';

export type GoogleCalendarListResponse = {
	accountEmail: string;
	calendars: GoogleCalendarListEntry[];
};

export type GoogleCalendarSelectionResponse = {
	accountEmail: string;
	selectedCalendar: GoogleCalendarListEntry;
	calendarURL: string;
};
