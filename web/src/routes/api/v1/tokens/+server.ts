import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { personalAccessTokens } from '$lib/server/control-plane';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const member = await callingMember(request, environmentOf(platform));
	return json({ tokens: await personalAccessTokens(member.record, member.memberID) });
};
