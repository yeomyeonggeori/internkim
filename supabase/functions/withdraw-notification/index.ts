import { callingAgent } from '../_shared/agent-caller.ts';
import { askedObject, json, refuse, serveRefusals } from '../_shared/http.ts';
import { membersOfCompanyByEmail, membersOfCompanyByExternalID } from '../_shared/member-directory.ts';
import { addressedIn, type AskedRecipients } from '../_shared/notify-recipients.ts';
import { withdrawFromMemberPhones } from '../_shared/notification-withdrawal.ts';
import { pushKeysFromVault } from '../_shared/push-keys.ts';

type WithdrawRequest = AskedRecipients & { messageID?: unknown };

Deno.serve(
	serveRefusals(async (request) => {
		if (request.method !== 'POST') refuse(405, 'POST only');
		const { client, companyID } = await callingAgent(request);

		const asked = (await askedObject(request)) as WithdrawRequest;
		const messageID = typeof asked.messageID === 'string' ? asked.messageID.trim() : '';
		if (!messageID) refuse(400, 'which message was taken back: its messageID');
		const addressed = addressedIn(asked);
		if (!addressed) refuse(400, 'whose phones: emails, or a platform and the externalIDs of its accounts');

		const apnsKey = (await pushKeysFromVault(client)).apns;
		if (!apnsKey) return json({ reached: 0 });

		const memberOf =
			addressed.by === 'email'
				? await membersOfCompanyByEmail(client, companyID)
				: await membersOfCompanyByExternalID(client, companyID, addressed.platform);
		const memberIDs = addressed.keys.map((key) => memberOf.get(key)).filter((id): id is string => Boolean(id));
		const nowInSeconds = Math.floor(Date.now() / 1000);

		let reached = 0;
		for (const memberID of memberIDs) {
			reached += await withdrawFromMemberPhones(client, memberID, messageID, apnsKey, nowInSeconds);
		}
		return json({ reached });
	})
);
