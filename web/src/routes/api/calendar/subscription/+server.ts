import { environmentOf } from '$lib/server/agent-request';
import { callingMember } from '$lib/server/member-request';
import { companyHasAFeedToken, forgetFeedToken, issueFeedToken } from '$lib/server/calendar/feed-token';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, platform }) => {
	const member = await callingMember(request, environmentOf(platform));
	return json({ registered: await companyHasAFeedToken(member.caller, member.companyID) });
};

export const POST: RequestHandler = async ({ request, url, platform }) => {
	const member = await callingMember(request, environmentOf(platform));
	const token = await issueFeedToken(member.caller);
	return json({ address: `${url.origin}/calendar/feed/${token}.ics` });
};

export const DELETE: RequestHandler = async ({ request, platform }) => {
	const member = await callingMember(request, environmentOf(platform));
	await forgetFeedToken(member.caller);
	return json({ forgotten: true });
};
