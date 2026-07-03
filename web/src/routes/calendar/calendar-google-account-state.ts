import type { CalendarAccountStatusResponse } from './calendar-layout-types';
import type { CalendarGoogleAccountText } from './text';

export function isWaitingForInitialGoogleAccountStatus(
	isLoadingAccountStatus: boolean,
	accountStatus: CalendarAccountStatusResponse | null
): boolean {
	return isLoadingAccountStatus && !accountStatus;
}

export function googleAccountStatusLabel(
	text: CalendarGoogleAccountText,
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): string {
	if (isWaitingForInitialGoogleAccountStatus(isLoadingAccountStatus, accountStatus)) return text.accountStatusLoading;
	if (accountStatusError) return text.accountStatusLoadFailed;
	if (!accountStatus?.connected) return text.googleCalendarDisconnected;
	if (accountStatus.needsReauth) return text.googleCalendarReauthRequired;
	if (accountStatus.accountEmail) {
		return text.googleCalendarConnectedTemplate.replace('{email}', accountStatus.accountEmail);
	}
	return text.googleCalendarConnected;
}

export function shouldShowGoogleOAuthAction(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (isWaitingForInitialGoogleAccountStatus(isLoadingAccountStatus, accountStatus) || accountStatusError) return false;
	if (accountStatus?.googleOAuthConfigured !== true) return false;
	return !accountStatus.connected || accountStatus.needsReauth;
}

export function shouldShowGoogleOAuthUnavailableHint(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (isWaitingForInitialGoogleAccountStatus(isLoadingAccountStatus, accountStatus) || accountStatusError) return false;
	if (accountStatus?.googleOAuthConfigured !== false) return false;
	return !accountStatus.connected || accountStatus.needsReauth;
}

export function shouldShowGoogleOAuthReadyHint(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (isWaitingForInitialGoogleAccountStatus(isLoadingAccountStatus, accountStatus) || accountStatusError) return false;
	if (accountStatus?.googleOAuthConfigured !== true) return false;
	return !accountStatus.connected && !accountStatus.needsReauth;
}

export function canManageGoogleOAuthAccount(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (isWaitingForInitialGoogleAccountStatus(isLoadingAccountStatus, accountStatus) || accountStatusError) return false;
	return accountStatus?.canManageGoogleOAuth === true;
}

export function shouldShowGoogleOAuthInitialUpload(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (!canManageGoogleOAuthAccount(accountStatus, accountStatusError, isLoadingAccountStatus)) return false;
	return accountStatus?.googleOAuthConfigured === false;
}

export function shouldShowGoogleOAuthReplacementUpload(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (!canManageGoogleOAuthAccount(accountStatus, accountStatusError, isLoadingAccountStatus)) return false;
	return accountStatus?.googleOAuthConfigured === true;
}

export function shouldShowGoogleOAuthSwitchAccountAction(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (!canManageGoogleOAuthAccount(accountStatus, accountStatusError, isLoadingAccountStatus)) return false;
	if (accountStatus?.googleOAuthConfigured !== true) return false;
	return accountStatus.connected && !accountStatus.needsReauth;
}

export function shouldShowGoogleCalendarSelector(
	accountStatus: CalendarAccountStatusResponse | null,
	accountStatusError: boolean,
	isLoadingAccountStatus: boolean
): boolean {
	if (!canManageGoogleOAuthAccount(accountStatus, accountStatusError, isLoadingAccountStatus)) return false;
	if (!accountStatus?.connected || accountStatus.needsReauth) return false;
	return accountStatus.googleOAuthConfigured === true;
}

export function googleOAuthActionLabel(
	text: CalendarGoogleAccountText,
	accountStatus: CalendarAccountStatusResponse | null
): string {
	if (accountStatus?.needsReauth) return text.googleCalendarReconnectAction;
	return text.googleCalendarConnectAction;
}

export function googleOAuthStartURLForReturnURL(returnURL: string): string {
	return `/calendar/oauth/google/start?returnTo=${encodeURIComponent(returnURL)}&switchAccount=true&popup=true`;
}

export function selectedGoogleCalendarLabel(accountStatus: CalendarAccountStatusResponse | null): string {
	return accountStatus?.selectedCalendarName || accountStatus?.selectedCalendarID || '';
}
