import { adminCallerOf, asMember, controlPlane, removeMember } from '$lib/server/control-plane';
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
	if (!caller) error(403, 'only an admin removes people');

	const body = (await request.json().catch(() => ({}))) as { memberID?: unknown; purge?: unknown };
	const memberID = typeof body.memberID === 'string' ? body.memberID.trim() : '';
	if (!memberID) error(400, 'which member to remove');
	if (memberID === caller.memberID) error(400, 'an admin does not remove themselves');

	const client = controlPlane({ projectURL, serviceRoleKey });
	const { wasRemoved } = await removeMember(client, caller.companyID, memberID, { purge: body.purge === true });
	return json({ memberID, wasRemoved });
};
