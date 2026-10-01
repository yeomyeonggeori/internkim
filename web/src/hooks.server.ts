import { building } from '$app/environment';
import { env } from '$env/dynamic/private';
import { redirect, type Handle } from '@sveltejs/kit';
import { movesToTheOneAddress, theOneAddressOf } from '$lib/server/company-host-redirect';
import { apiReferenceHomeFor } from '$lib/server/api-reference-redirect';
import { defaultZone } from '$lib/server/fleet-domain';
import { asksForConsent, unframeableHeaders } from '$lib/server/consent-framing';

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
		vapidPublicKey: env.VAPID_PUBLIC_KEY ?? '',
		gatewayURL: env.GATEWAY_URL ?? ''
	};

	const zone = env.CLOUDFLARE_DOMAIN || defaultZone;

	const referenceHome = apiReferenceHomeFor(event.url.pathname, zone);
	if (referenceHome) redirect(308, referenceHome);

	if (!building && movesToTheOneAddress({ hostname: event.url.hostname, zone, pathname: event.url.pathname })) {
		redirect(308, `https://${theOneAddressOf(zone)}${event.url.pathname}${event.url.search}`);
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

	if (asksForConsent(event.url.pathname)) {
		for (const [name, value] of Object.entries(unframeableHeaders)) response.headers.set(name, value);
	}

	return response;
};
