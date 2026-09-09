import { callingAgent } from '../_shared/agent-caller.ts';
import { announceEventReminders } from '../_shared/announce-event-reminder.ts';
import { json, refuse, serveRefusals } from '../_shared/http.ts';
import { pushKeysFromVault, reachesSomeDevice } from '../_shared/push-keys.ts';

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client } = await callingAgent(request);
		const pushKeys = await pushKeysFromVault(client);
		if (!reachesSomeDevice(pushKeys)) refuse(503, 'this deployment holds no push keys, so it cannot send notifications yet');

		const now = new Date();
		return json(await announceEventReminders(client, now, pushKeys, Math.floor(now.getTime() / 1000)));
	})
);
