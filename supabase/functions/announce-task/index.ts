import { announceTaskChange, type TaskChange } from '../_shared/announce-task.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { callingMember } from '../_shared/member-caller.ts';
import { pushKeysFromVault, reachesSomeDevice } from '../_shared/push-keys.ts';

type AnnounceRequest = { taskID?: unknown; moved?: unknown; askedMemberIDs?: unknown };

function changeAsked(asked: AnnounceRequest): TaskChange {
	const taskID = typeof asked.taskID === 'string' ? asked.taskID.trim() : '';
	if (!taskID) refuse(400, 'taskID required');
	if (asked.moved !== undefined && typeof asked.moved !== 'boolean') refuse(400, 'moved must be true or false');
	if (asked.askedMemberIDs !== undefined && !isListOfStrings(asked.askedMemberIDs)) {
		refuse(400, 'askedMemberIDs must be a list of member IDs');
	}
	return {
		taskID,
		moved: asked.moved ?? asked.askedMemberIDs === undefined,
		askedMemberIDs: asked.askedMemberIDs ?? []
	};
}

function isListOfStrings(value: unknown): value is string[] {
	return Array.isArray(value) && value.every((item) => typeof item === 'string');
}

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { caller, record, memberID } = await callingMember(request);
		const pushKeys = await pushKeysFromVault(record);
		if (!reachesSomeDevice(pushKeys)) refuse(503, 'this deployment holds no push keys, so it cannot send notifications yet');

		const change = changeAsked((await askedObject(request)) as AnnounceRequest);
		return json(await announceTaskChange(caller, record, memberID, change, pushKeys, Math.floor(Date.now() / 1000)));
	})
);
