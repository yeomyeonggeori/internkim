import { announceClock, announceLeaveRequest } from '../_shared/announce-attendance.ts';
import { json, refuse, serveRefusals } from '../_shared/http.ts';
import { callingMember } from '../_shared/member-caller.ts';
import { vapidKeysFromVault } from '../_shared/vapid-from-vault.ts';

type AnnounceRequest = { what?: unknown };

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { caller, record, memberID } = await callingMember(request);
		const vapid = await vapidKeysFromVault(record);
		if (!vapid) refuse(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');
		const nowInSeconds = Math.floor(Date.now() / 1000);

		const asked = (await request.json().catch(() => ({}))) as AnnounceRequest;
		if (asked.what === 'clock') {
			return json(await announceClock(caller, record, memberID, vapid, nowInSeconds));
		}
		if (asked.what === 'leave') {
			return json(await announceLeaveRequest(caller, record, memberID, vapid, nowInSeconds));
		}
		refuse(400, 'what must be clock or leave');
	})
);
