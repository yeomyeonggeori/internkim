import { announceTaskMove } from '../_shared/announce-task.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { callingMember } from '../_shared/member-caller.ts';
import { pushKeysFromVault, reachesSomeDevice } from '../_shared/push-keys.ts';

type AnnounceRequest = { taskID?: unknown };

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { caller, record, memberID } = await callingMember(request);
		const pushKeys = await pushKeysFromVault(record);
		if (!reachesSomeDevice(pushKeys)) refuse(503, 'this deployment holds no push keys, so it cannot send notifications yet');

		const asked = (await askedObject(request)) as AnnounceRequest;
		const taskID = typeof asked.taskID === 'string' ? asked.taskID.trim() : '';
		if (!taskID) refuse(400, 'taskID required');

		return json(
			await announceTaskMove(caller, record, memberID, taskID, pushKeys, Math.floor(Date.now() / 1000))
		);
	})
);
