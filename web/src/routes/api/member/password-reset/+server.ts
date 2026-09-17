import { adminCallerOf, asMember, controlPlane, planeCredentialsOf, resetMemberPassword } from '$lib/server/control-plane';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, plane);

	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, 'only an admin resets a password');

	const body = (await request.json().catch(() => ({}))) as { memberID?: unknown };
	const memberID = typeof body.memberID === 'string' ? body.memberID.trim() : '';
	if (!memberID) error(400, 'which member to reset');

	const client = controlPlane(plane);
	const { data: member } = await client
		.from('member')
		.select('id')
		.eq('company_id', caller.companyID)
		.eq('id', memberID)
		.maybeSingle();
	if (!member) error(404, 'no such member in this company');

	return json(await resetMemberPassword(client, member.id));
};
