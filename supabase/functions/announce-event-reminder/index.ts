import { callingAgent } from '../_shared/agent-caller.ts';
import { announceEventReminders } from '../_shared/announce-event-reminder.ts';
import { json, refuse, serveRefusals } from '../_shared/http.ts';
import { vapidKeysFromVault } from '../_shared/vapid-from-vault.ts';

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client } = await callingAgent(request);
		const vapid = await vapidKeysFromVault(client);
		if (!vapid) refuse(503, 'this deployment has no VAPID keys, so it cannot send notifications yet');

		const now = new Date();
		return json(await announceEventReminders(client, now, vapid, Math.floor(now.getTime() / 1000)));
	})
);
