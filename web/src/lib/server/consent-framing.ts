import { routePathOf } from '$lib/company-path';

export const unframeableHeaders = {
	'Content-Security-Policy': "frame-ancestors 'none'",
	'X-Frame-Options': 'DENY'
};

export function asksForConsent(pathname: string): boolean {
	const routePath = routePathOf(pathname);
	return routePath === '/oauth' || routePath.startsWith('/oauth/');
}
