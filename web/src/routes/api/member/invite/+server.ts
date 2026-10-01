import {
	addMember,
	AddressBelongsToAnotherCompany,
	adminCallerOf,
	AlreadyAMember,
	asMember,
	controlPlane,
	inviteMember,
	planeCredentialsOf
} from '$lib/server/control-plane';
import type { SupabaseClient } from '@supabase/supabase-js';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { signedInAccessTokenOf } from '$lib/server/member-request';

export const POST: RequestHandler = async ({ request, platform }) => {
	const plane = planeCredentialsOf({ ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) });
	if (!plane) error(500, 'the central plane is not configured');

	const accessToken = await signedInAccessTokenOf(request, plane);

	const caller = await adminCallerOf(asMember(plane, accessToken));
	if (!caller) error(403, 'only an admin invites people');

	const body = (await request.json().catch(() => ({}))) as { email?: unknown; name?: unknown; isAdmin?: unknown };
	const email = typeof body.email === 'string' ? body.email.trim().toLowerCase() : '';
	const name = typeof body.name === 'string' ? body.name.trim() : '';
	if (!email.includes('@')) error(400, 'an address is required');
	if (!name) error(400, 'a name is required');

	const client = controlPlane(plane);
	const memberID = await memberAddedAt(client, caller.companyID, email, { isAdmin: body.isAdmin === true, name });
	const invitation = await inviteMember(client, memberID);
	return json(invitation);
};

async function memberAddedAt(
	client: SupabaseClient,
	companyID: string,
	email: string,
	options: { isAdmin: boolean; name: string }
): Promise<string> {
	try {
		return await addMember(client, companyID, email, options);
	} catch (refusal) {
		if (refusal instanceof AddressBelongsToAnotherCompany) error(409, 'that address belongs to another company');
		if (refusal instanceof AlreadyAMember) error(409, 'that person is already a member of this company');
		throw refusal;
	}
}
