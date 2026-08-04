import { addMember, adminCallerOf, asMember, controlPlane, inviteMember } from '$lib/server/control-plane';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = (platform?.env ?? process.env) as Record<string, string | undefined>;
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');

	const caller = await adminCallerOf(asMember({ projectURL, publishableKey }, accessToken));
	if (!caller) error(403, 'only an admin invites people');

	const body = (await request.json().catch(() => ({}))) as { email?: unknown; isAdmin?: unknown };
	const email = typeof body.email === 'string' ? body.email.trim().toLowerCase() : '';
	if (!email.includes('@')) error(400, 'an address is required');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const { data: existing } = await client.from('member').select('company_id').eq('email', email).maybeSingle();
	if (existing && existing.company_id !== caller.companyID) error(409, 'that address belongs to another company');

	const memberID = await addMember(client, caller.companyID, email, { isAdmin: body.isAdmin === true });
	const invitation = await inviteMember(client, memberID);
	return json(invitation);
};
