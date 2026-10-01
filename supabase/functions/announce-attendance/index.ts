import { announceClock, announceLeaveRequest } from '../_shared/announce-attendance.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { callingMember } from '../_shared/member-caller.ts';
import { pushKeysFromVault, reachesSomeDevice } from '../_shared/push-keys.ts';

type AnnounceRequest = { what?: unknown; colleagues?: unknown };

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { caller, record, memberID } = await callingMember(request);
		const pushKeys = await pushKeysFromVault(record);
		if (!reachesSomeDevice(pushKeys)) refuse(503, 'this deployment holds no push keys, so it cannot send notifications yet');
		const nowInSeconds = Math.floor(Date.now() / 1000);

		const asked = (await askedObject(request)) as AnnounceRequest;
		if (asked.what === 'clock') {
			return json(await announceClock(caller, record, memberID, pushKeys, nowInSeconds, asked.colleagues !== false));
		}
		if (asked.what === 'leave') {
			return json(await announceLeaveRequest(caller, record, memberID, pushKeys, nowInSeconds));
		}
		refuse(400, 'what must be clock or leave');
	})
);
