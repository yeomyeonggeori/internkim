import { env } from '$env/dynamic/private';
import { asMember, claimMemberFor, controlPlane } from '$lib/server/control-plane';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';

export const GET: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, { projectURL, serviceRoleKey });

	const { data: account } = await asMember({ projectURL, publishableKey }, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const claimed = await claimMemberFor(controlPlane({ projectURL, serviceRoleKey }), account.user.id, email);
	if (!claimed) return json({ member: null });
	return json({ member: claimed });
};
