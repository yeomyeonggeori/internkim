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
	canManageGoogleOAuth: boolean;
};
