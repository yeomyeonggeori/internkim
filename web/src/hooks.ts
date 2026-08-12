import type { Reroute } from '@sveltejs/kit';
import { routePathOf } from '$lib/company-path';

export const reroute: Reroute = ({ url }) => routePathOf(url.pathname);
