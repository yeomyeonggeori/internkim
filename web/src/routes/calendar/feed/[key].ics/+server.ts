import { environmentOf } from '$lib/server/agent-request';
import { calendarFeedOf } from '$lib/server/calendar-feed/feed-record';
import { icsDocumentOf } from '$lib/server/calendar-feed/ics-document';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ params, url, platform }) => {
	const environment = environmentOf(platform);
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey =
		environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the control plane is not configured');

	const feed = await calendarFeedOf({ projectURL, serviceRoleKey }, params.key);
	if (!feed) error(404, 'no calendar answers to that address');

	return new Response(icsDocumentOf(feed, url.hostname), {
		headers: {
			'Content-Type': 'text/calendar; charset=utf-8',
			'Content-Disposition': 'inline; filename="internkim.ics"',
			'Cache-Control': 'no-store'
		}
	});
};
