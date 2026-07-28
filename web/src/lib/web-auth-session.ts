export const webAuthSessionDependency = 'app:session';

export type WebAuthSession = {
	authenticated: boolean;
	email: string;
	mattermostLoginURL: string;
	cloudflareLoginURL: string;
	isUnavailable: boolean;
};

type SessionResponse = {
	authenticated?: boolean;
	email?: string;
	loginURL?: string;
	mattermostLoginURL?: string;
	cloudflareLoginURL?: string;
};

export function mattermostLoginURLFor(returnPath: string): string {
	return `/auth/mattermost/start?return=${encodeURIComponent(returnPath)}`;
}

export function cloudflareLoginURLFor(returnPath: string): string {
	return `/auth/cloudflare/start?return=${encodeURIComponent(returnPath)}`;
}

export function signedOutSession(returnPath: string, isUnavailable: boolean): WebAuthSession {
	return {
		authenticated: false,
		email: '',
		mattermostLoginURL: mattermostLoginURLFor(returnPath),
		cloudflareLoginURL: cloudflareLoginURLFor(returnPath),
		isUnavailable
	};
}

export function webAuthSessionFrom(response: SessionResponse, returnPath: string): WebAuthSession {
	return {
		authenticated: Boolean(response.authenticated),
		email: response.email ?? '',
		mattermostLoginURL: response.mattermostLoginURL || response.loginURL || mattermostLoginURLFor(returnPath),
		cloudflareLoginURL: response.cloudflareLoginURL || cloudflareLoginURLFor(returnPath),
		isUnavailable: false
	};
}
