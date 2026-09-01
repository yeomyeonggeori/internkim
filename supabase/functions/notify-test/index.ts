import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { callingMember } from '../_shared/member-caller.ts';
import { pushToMemberDevices } from '../_shared/push-to-member-devices.ts';
import { selfTestNotification, type SelfTest } from '../_shared/self-test-notification.ts';
import { vapidKeysFromVault } from '../_shared/vapid-from-vault.ts';

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { caller, record, memberID } = await callingMember(request);
		const vapid = await vapidKeysFromVault(record);
		if (!vapid) refuse(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');

		const asked = (await askedObject(request)) as SelfTest;
		const { reached, pruned } = await pushToMemberDevices(
			caller,
			memberID,
			selfTestNotification(asked),
			vapid,
			Math.floor(Date.now() / 1000)
		);
		return json({ reached, pruned });
	})
);
