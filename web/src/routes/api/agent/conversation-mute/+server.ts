import { error, json } from '@sveltejs/kit';
import { z } from 'zod';
import type { RequestHandler } from './$types';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { memberBelongsToCompany } from '$lib/server/member-credential';

const conversationMuteSchema = z.strictObject({
	memberID: z.uuid(),
	conversationID: z.string().trim().min(1).max(256)
});

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = conversationMuteSchema.safeParse(await request.json().catch(() => ({})));
	if (!asked.success) error(400, 'a mute names the member and the conversation they stop being told about');
	if (!(await memberBelongsToCompany(client, asked.data.memberID, companyID))) {
		error(403, 'that member belongs to another company');
	}

	const muted = await client.rpc('conversation_mute_for', {
		muted_member: asked.data.memberID,
		conversation: asked.data.conversationID
	});
	if (muted.error) return json({ error: muted.error.message }, { status: 502 });
	return json({ muted: true });
};
