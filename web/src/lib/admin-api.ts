const accessReauthenticationStorageKey = 'cfAccessReauthAt';
const accessReauthenticationCooldownMs = 8000;

export async function adminApiFetch(input: string, init: RequestInit = {}): Promise<Response> {
	const response = await fetch(input, { ...init, credentials: 'include', redirect: 'manual' });
	if (response.type === 'opaqueredirect') {
		requestAccessReauthentication();
		throw new Error('cloudflare-access-reauthentication-required');
	}
	return response;
}

function requestAccessReauthentication(): void {
	if (typeof window === 'undefined') return;
	const now = Date.now();
	const lastAttempt = Number(window.sessionStorage.getItem(accessReauthenticationStorageKey) ?? '0');
	if (now - lastAttempt < accessReauthenticationCooldownMs) return;
	window.sessionStorage.setItem(accessReauthenticationStorageKey, String(now));
	window.location.reload();
}
