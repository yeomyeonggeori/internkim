export const webAuthSessionDependency = 'app:session';

export type WebAuthSession = {
	authenticated: boolean;
	email: string;
	image: string;
	canViewTasks: boolean;
	isPocSuperAdmin: boolean;
	mattermostLoginURL: string;
	cloudflareLoginURL: string;
	isUnavailable: boolean;
};

type SessionResponse = {
	authenticated?: boolean;
	email?: string;
	image?: string;
	canViewTasks?: boolean;
	isPocSuperAdmin?: boolean;
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
		image: '',
		canViewTasks: false,
		isPocSuperAdmin: false,
		mattermostLoginURL: mattermostLoginURLFor(returnPath),
		cloudflareLoginURL: cloudflareLoginURLFor(returnPath),
		isUnavailable
	};
}

export function webAuthSessionFrom(response: SessionResponse, returnPath: string): WebAuthSession {
	return {
		authenticated: Boolean(response.authenticated),
		email: response.email ?? '',
		image: response.image ?? '',
		canViewTasks: response.canViewTasks === true,
		isPocSuperAdmin: response.isPocSuperAdmin === true,
		mattermostLoginURL: response.mattermostLoginURL || response.loginURL || mattermostLoginURLFor(returnPath),
		cloudflareLoginURL: response.cloudflareLoginURL || cloudflareLoginURLFor(returnPath),
		isUnavailable: false
	};
}
