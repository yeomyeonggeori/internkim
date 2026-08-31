import { env } from '$env/dynamic/private';
import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { asMember, claimMemberFor, controlPlane, isPersonalAccessToken } from './control-plane';

function environmentOf(platform: App.Platform | undefined) {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');
	return { projectURL, publishableKey, serviceRoleKey };
}

// Tokens are made and revoked signed in. A token that could make another here
// would raise itself past the rules the public API keeps at /v1/token.
export async function memberSignedIn(request: Request, platform: App.Platform | undefined): Promise<string> {
	const { projectURL, publishableKey, serviceRoleKey } = environmentOf(platform);

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');
	if (isPersonalAccessToken(accessToken)) error(403, 'tokens are made and revoked signed in, or through /v1/token');

	const { data: account } = await asMember({ projectURL, publishableKey }, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const claimed = await claimMemberFor(controlPlane({ projectURL, serviceRoleKey }), account.user.id, email);
	if (!claimed) error(403, 'no member here goes by that address');
	return claimed.memberID;
}

export function tokenStore(platform: App.Platform | undefined): SupabaseClient {
	const { projectURL, serviceRoleKey } = environmentOf(platform);
	return controlPlane({ projectURL, serviceRoleKey });
}
