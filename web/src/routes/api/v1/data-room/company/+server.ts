import { json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { companyShownTo } from '$lib/server/data-room-company';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const member = await callingMember(request, environmentOf(platform));
	return json(await companyShownTo(member.caller, member.companyID), { headers: { 'Cache-Control': 'no-store' } });
};
