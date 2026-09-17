import { environmentOf } from '$lib/server/agent-request';
import { calendarFeedForToken } from '$lib/server/calendar/feed';
import { planeCredentialsOf } from '$lib/server/control-plane';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, platform }) => {
	const plane = planeCredentialsOf(environmentOf(platform));
	if (!plane) error(500, 'the control plane is not configured');

	const feed = await calendarFeedForToken(plane, params.token, new Date());
	if (feed === null) error(404, 'no calendar answers to that address');

	return new Response(feed, {
		headers: {
			'Content-Type': 'text/calendar; charset=utf-8',
			'Cache-Control': 'private, no-store'
		}
	});
};
