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
	googleOAuthConfigured: boolean;
	canManageGoogleOAuth: boolean;
};

export type GoogleCalendarListEntry = {
	calendarID: string;
	summary: string;
	accessRole: string;
	primary: boolean;
	backgroundColor?: string;
};

export type GoogleCalendarListResponse = {
	accountEmail: string;
	calendars: GoogleCalendarListEntry[];
};

export type GoogleCalendarSelectionResponse = {
	accountEmail: string;
	selectedCalendar: GoogleCalendarListEntry;
	calendarURL: string;
};
