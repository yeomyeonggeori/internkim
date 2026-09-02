import { adminCallerOf, asMember, controlPlane, resetMemberPassword } from '$lib/server/control-plane';
import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, { projectURL, serviceRoleKey });

	const caller = await adminCallerOf(asMember({ projectURL, publishableKey }, accessToken));
	if (!caller) error(403, 'only an admin resets a password');

	const body = (await request.json().catch(() => ({}))) as { memberID?: unknown };
	const memberID = typeof body.memberID === 'string' ? body.memberID.trim() : '';
	if (!memberID) error(400, 'which member to reset');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const { data: member } = await client
		.from('member')
		.select('id')
		.eq('company_id', caller.companyID)
		.eq('id', memberID)
		.maybeSingle();
	if (!member) error(404, 'no such member in this company');

	return json(await resetMemberPassword(client, member.id));
};
