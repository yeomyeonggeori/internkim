import {
	addMember,
	AddressBelongsToAnotherCompany,
	adminCallerOf,
	AlreadyAMember,
	asMember,
	controlPlane,
	inviteMember,
	isAboveCaller,
	promoteToAdministrator,
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

	const callerClient = asMember(plane, accessToken);
	const caller = await adminCallerOf(callerClient);
	if (!caller) error(403, 'only an admin invites people');

	const body = (await request.json().catch(() => ({}))) as { email?: unknown; name?: unknown; isAdmin?: unknown };
	const email = typeof body.email === 'string' ? body.email.trim().toLowerCase() : '';
	const name = typeof body.name === 'string' ? body.name.trim() : '';
	if (!email.includes('@')) error(400, 'an address is required');
	if (!name) error(400, 'a name is required');

	const client = controlPlane(plane);
	const memberID = await memberAddedAt(client, caller.companyID, email, { name });
	if (await isAboveCaller(callerClient, caller.memberID, memberID)) {
		error(403, 'an administrator invites nobody above their own clearance');
	}
	if (body.isAdmin === true) await promoteToAdministrator(callerClient, memberID);
	const invitation = await inviteMember(client, memberID);
	return json(invitation);
};

async function memberAddedAt(
	client: SupabaseClient,
	companyID: string,
	email: string,
	options: { name: string }
): Promise<string> {
	try {
		return await addMember(client, companyID, email, options);
	} catch (refusal) {
		if (refusal instanceof AddressBelongsToAnotherCompany) error(409, 'that address belongs to another company');
		if (refusal instanceof AlreadyAMember) error(409, 'that person is already a member of this company');
		throw refusal;
	}
}
