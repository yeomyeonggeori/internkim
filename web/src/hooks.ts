import type { Reroute } from '@sveltejs/kit';
import { routePathOf } from '$lib/company-path';

export const reroute: Reroute = ({ url }) => {
	if (url.pathname === '/v1' || url.pathname.startsWith('/v1/')) return `/api${url.pathname}`;
	return routePathOf(url.pathname);
};
