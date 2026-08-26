import { error, json } from '@sveltejs/kit';
import { environmentOf, type Environment } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { announceClock, announceLeaveRequest } from '$lib/server/announce-attendance';
import type { VapidKeys } from '$lib/server/web-push-vapid';
import type { RequestHandler } from './$types';

type AnnounceRequest = { what?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { caller, record, memberID } = await callingMember(request, environment);
	const vapid = vapidKeys(environment);
	const nowInSeconds = Math.floor(Date.now() / 1000);

	const asked = (await request.json().catch(() => ({}))) as AnnounceRequest;
	if (asked.what === 'clock') {
		return json(await announceClock(caller, record, memberID, vapid, nowInSeconds));
	}
	if (asked.what === 'leave') {
		return json(await announceLeaveRequest(caller, record, memberID, vapid, nowInSeconds));
	}
	error(400, 'what must be clock or leave');
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
