import { error, json } from '@sveltejs/kit';
import { environmentOf, type Environment } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { announceTaskMove } from '$lib/server/announce-task';
import type { VapidKeys } from '$lib/server/web-push-vapid';
import type { RequestHandler } from './$types';

type AnnounceRequest = { taskID?: unknown };

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const { caller, record, memberID } = await callingMember(request, environment);
	const vapid = vapidKeys(environment);

	const asked = (await request.json().catch(() => ({}))) as AnnounceRequest;
	const taskID = typeof asked.taskID === 'string' ? asked.taskID.trim() : '';
	if (!taskID) error(400, 'taskID required');

	return json(await announceTaskMove(caller, record, memberID, taskID, vapid, Math.floor(Date.now() / 1000)));
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
