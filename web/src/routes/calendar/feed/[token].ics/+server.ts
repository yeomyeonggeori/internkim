import { environmentOf } from '$lib/server/agent-request';
import { calendarFeedForToken } from '$lib/server/calendar/feed';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, platform }) => {
	const environment = environmentOf(platform);
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the control plane is not configured');

	const feed = await calendarFeedForToken(
		{ projectURL, publishableKey, serviceRoleKey },
		params.token,
		new Date()
	);
	if (feed === null) error(404, 'no calendar answers to that address');

	return new Response(feed, {
		headers: {
			'Content-Type': 'text/calendar; charset=utf-8',
			'Cache-Control': 'private, no-store'
		}
	});
};
