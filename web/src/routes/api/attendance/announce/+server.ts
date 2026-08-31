import { error, json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { announceClock, announceLeaveRequest } from '$lib/server/announce-attendance';
import { vapidKeysInUse } from '$lib/server/vapid-keys';
import type { RequestHandler } from './$types';

type AnnounceRequest = { what?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { caller, record, memberID } = await callingMember(request, environment);
	const vapid = await vapidKeysInUse(record, environment);
	if (!vapid) error(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');
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
