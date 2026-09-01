import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { announceTheDay } from '$lib/server/announce-the-day';
import { vapidKeysInUse } from '$lib/server/vapid-keys';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { client } = await callingAgent(request, environment);
	const vapid = await vapidKeysInUse(client, environment);
	if (!vapid) error(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');
	const now = new Date();
	return json(await announceTheDay(client, now, vapid, Math.floor(now.getTime() / 1000)));
};
