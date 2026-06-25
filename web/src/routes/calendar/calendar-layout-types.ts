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
	lastAuthError?: string;
	needsReauth: boolean;
	googleOAuthConfigured: boolean;
};

export type CalendarEvent = {
	id?: string;
	title?: string;
	startISO: string;
	endISO: string;
	isAllDay: boolean;
};

export type CalendarEventsResponse = {
	events?: CalendarEvent[];
};

export type CalendarSource = {
	id: string;
	label: string;
	count: string;
	isChecked: boolean;
	colorClass: string;
};
