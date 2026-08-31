import { error, json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { announceTaskMove } from '$lib/server/announce-task';
import { vapidKeysInUse } from '$lib/server/vapid-keys';
import type { RequestHandler } from './$types';

type AnnounceRequest = { taskID?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { caller, record, memberID } = await callingMember(request, environment);
	const vapid = await vapidKeysInUse(record, environment);
	if (!vapid) error(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');

	const asked = (await request.json().catch(() => ({}))) as AnnounceRequest;
	const taskID = typeof asked.taskID === 'string' ? asked.taskID.trim() : '';
	if (!taskID) error(400, 'taskID required');

	return json(await announceTaskMove(caller, record, memberID, taskID, vapid, Math.floor(Date.now() / 1000)));
};
