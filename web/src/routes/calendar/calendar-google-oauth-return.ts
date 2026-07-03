export const googleOAuthReturnStorageKey = 'internkim:calendar:google-oauth-return';
export const googleOAuthReturnMessageType = 'internkim:calendar:google-oauth-return';

export type GoogleOAuthReturnStatus = 'connected' | 'failed';

type GoogleOAuthReturnSignal = {
	status: GoogleOAuthReturnStatus;
	issuedAt: number;
};

export function googleOAuthReturnStatusFromURL(returnURL: URL): GoogleOAuthReturnStatus | null {
	return googleOAuthReturnStatusFromValue(returnURL.searchParams.get('googleOAuth'));
}

export function notifyGoogleOAuthReturn(status: GoogleOAuthReturnStatus): void {
	const signal: GoogleOAuthReturnSignal = {
		status,
		issuedAt: Date.now()
	};
	try {
		window.localStorage.setItem(googleOAuthReturnStorageKey, JSON.stringify(signal));
	} catch {
		return;
	}
}

export function googleOAuthReturnStatusFromStorageEvent(event: StorageEvent): GoogleOAuthReturnStatus | null {
	if (event.key !== googleOAuthReturnStorageKey) return null;
	if (!event.newValue) return null;
	return parseGoogleOAuthReturnSignal(event.newValue);
}

export function googleOAuthReturnStatusFromMessageEvent(event: MessageEvent, expectedOrigin: string): GoogleOAuthReturnStatus | null {
	if (event.origin !== expectedOrigin) return null;
	if (!isRecord(event.data) || event.data.type !== googleOAuthReturnMessageType) return null;
	return googleOAuthReturnStatusFromValue(event.data.status);
}

function parseGoogleOAuthReturnSignal(value: string): GoogleOAuthReturnStatus | null {
	let parsedValue: unknown;
	try {
		parsedValue = JSON.parse(value);
	} catch {
		return null;
	}
	if (!isRecord(parsedValue) || typeof parsedValue.issuedAt !== 'number') return null;
	return googleOAuthReturnStatusFromValue(parsedValue.status);
}

function googleOAuthReturnStatusFromValue(value: unknown): GoogleOAuthReturnStatus | null {
	if (value === 'connected' || value === 'failed') return value;
	return null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null;
}
