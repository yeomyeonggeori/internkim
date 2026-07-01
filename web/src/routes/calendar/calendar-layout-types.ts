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
