import { withReturnPath } from '$lib/return-path';

export const webAuthSessionDependency = 'app:session';

export type WebAuthSession = {
	authenticated: boolean;
	email: string;
	image: string;
	canViewTasks: boolean;
	cloudflareLoginURL: string;
	isUnavailable: boolean;
};

type SessionResponse = {
	authenticated?: boolean;
	email?: string;
	image?: string;
	canViewTasks?: boolean;
	cloudflareLoginURL?: string;
};

export function cloudflareLoginURLFor(returnPath: string): string {
	return withReturnPath('/auth/cloudflare/start', returnPath);
}

export function signedOutSession(returnPath: string, isUnavailable: boolean): WebAuthSession {
	return {
		authenticated: false,
		email: '',
		image: '',
		canViewTasks: false,
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
		cloudflareLoginURL: response.cloudflareLoginURL || cloudflareLoginURLFor(returnPath),
		isUnavailable: false
	};
}
