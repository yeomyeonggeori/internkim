import { addMember, adminCallerOf, asMember, controlPlane, inviteMember, planeCredentialsOf } from '$lib/server/control-plane';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, plane);

	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, 'only an admin invites people');

	const body = (await request.json().catch(() => ({}))) as { email?: unknown; name?: unknown; isAdmin?: unknown };
	const email = typeof body.email === 'string' ? body.email.trim().toLowerCase() : '';
	const name = typeof body.name === 'string' ? body.name.trim() : '';
	if (!email.includes('@')) error(400, 'an address is required');
	if (!name) error(400, 'a name is required');

	const client = controlPlane(plane);
	const { data: existing } = await client.from('member').select('company_id').eq('email', email).maybeSingle();
	if (existing && existing.company_id !== caller.companyID) error(409, 'that address belongs to another company');

	const memberID = await addMember(client, caller.companyID, email, { isAdmin: body.isAdmin === true, name });
	const invitation = await inviteMember(client, memberID);
	return json(invitation);
};
