import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf, type Environment } from '$lib/server/agent-request';
import { announceTheDay } from '$lib/server/announce-the-day';
import type { VapidKeys } from '$lib/server/web-push-vapid';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { client } = await callingAgent(request, environment);
	const vapid = vapidKeys(environment);
	const now = new Date();
	return json(await announceTheDay(client, now, vapid, Math.floor(now.getTime() / 1000)));
};

function vapidKeys(environment: Environment): VapidKeys {
	const publicKey = environment.VAPID_PUBLIC_KEY ?? '';
	const privateKey = environment.VAPID_PRIVATE_KEY ?? '';
	const subject = environment.VAPID_SUBJECT ?? '';
	if (!publicKey || !privateKey || !subject) {
		error(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');
	}
	return { publicKey, privateKey, subject };
}
