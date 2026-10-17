import { json } from '@sveltejs/kit';
import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { companyShownTo } from '$lib/server/data-room-company';
import { dataRoomPlane } from '$lib/server/data-room-link-request';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const environment = environmentOf(platform);
	const member = await callingMember(request, environment);
	return json(await companyShownTo(member.caller, member.companyID, dataRoomPlane(environment)), { headers: { 'Cache-Control': 'no-store' } });
};
