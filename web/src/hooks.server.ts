import { building } from '$app/environment';
import { env } from '$env/dynamic/private';
import { redirect, type Handle } from '@sveltejs/kit';
import { sendsHomeToFlow } from '$lib/server/home-redirect';
import { appHostnameOf, movesToTheOneAddress } from '$lib/server/company-host-redirect';

export const handle: Handle = async ({ event, resolve }) => {
	if (event.request.method === 'OPTIONS') {
		return new Response(null, {
			headers: {
				'Access-Control-Allow-Origin': '*',
				'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
				'Access-Control-Allow-Headers': 'Content-Type'
			}
		});
	}

	const centralPlane = {
		projectURL: env.SUPABASE_URL ?? '',
		publishableKey: env.SUPABASE_PUBLISHABLE_KEY ?? '',
		vapidPublicKey: env.VAPID_PUBLIC_KEY ?? ''
	};

	const zone = env.CF_DOMAIN ?? '';
	if (!building && movesToTheOneAddress({ hostname: event.url.hostname, zone })) {
		redirect(308, `https://${appHostnameOf(zone)}${event.url.pathname}${event.url.search}`);
	}

	if (sendsHomeToFlow({ isBuilding: building, ...centralPlane, pathname: event.url.pathname })) {
		redirect(307, '/flow/');
	}

	const response = await resolve(event, {
		transformPageChunk: ({ html }) =>
			html.replace(
				'<script id="central-plane" type="application/json">{}</script>',
				`<script id="central-plane" type="application/json">${JSON.stringify(centralPlane)}</script>`
			)
	});

	if (event.url.pathname.startsWith('/api/')) {
		response.headers.set('Access-Control-Allow-Origin', '*');
	}

	return response;
};
