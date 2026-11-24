import { callingAgent } from '../_shared/agent-caller.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { clearReadOnMemberDevices, isMemberOfCompany } from '../_shared/notification-read-clearing.ts';
import { pushKeysFromVault } from '../_shared/push-keys.ts';

type ClearRequest = { memberID?: unknown; tag?: unknown };

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client, companyID } = await callingAgent(request);

		const asked = (await askedObject(request)) as ClearRequest;
		const memberID = typeof asked.memberID === 'string' ? asked.memberID.trim() : '';
		const tag = typeof asked.tag === 'string' ? asked.tag.trim() : '';
		if (!memberID || !tag) refuse(400, 'whose devices and which conversation: a memberID and the tag its notifications carry');
		if (!(await isMemberOfCompany(client, memberID, companyID))) refuse(404, 'no member of this company has that memberID');

		const reached = await clearReadOnMemberDevices(client, memberID, tag, await pushKeysFromVault(client), Math.floor(Date.now() / 1000));
		return json({ reached });
	})
);
