import { env } from '$env/dynamic/private';
import { error, json } from '@sveltejs/kit';
import { asMember, controlPlane } from '$lib/server/control-plane';
import { memberCredential } from '$lib/server/member-credential';
import type { RequestHandler } from './$types';

export const GET: RequestHandler = async ({ request, url, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) {
		error(500, 'the control plane is not configured');
	}

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');

	const caller = asMember({ projectURL, publishableKey }, accessToken);
	const { data: account } = await caller.auth.getUser();
	if (!account.user) error(401, 'sign in first');

	const member = await caller
		.from('member')
		.select('id')
		.eq('user_id', account.user.id)
		.maybeSingle<{ id: string }>();
	if (member.error) error(500, member.error.message);
	if (!member.data) error(403, 'refused');

	const kind = url.searchParams.get('kind') ?? 'mattermost';
	const credential = await memberCredential(
		controlPlane({ projectURL, serviceRoleKey }),
		member.data.id,
		kind,
	);
	if (!credential) error(404, 'no messenger credential yet');
	return json(credential);
};
