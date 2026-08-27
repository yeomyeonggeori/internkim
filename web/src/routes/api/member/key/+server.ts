import { env } from '$env/dynamic/private';
import {
	asMember,
	claimMemberFor,
	controlPlane,
	forgetPersonalKey,
	isPersonalKey,
	issuePersonalKey,
	personalKeys,
} from '$lib/server/control-plane';
import { fullPublicAPIPermission, publicAPIPermissionOf } from '$lib/public-api-permission';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

function environmentOf(platform: App.Platform | undefined) {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');
	return { projectURL, publishableKey, serviceRoleKey };
}

async function memberOf(request: Request, platform: App.Platform | undefined): Promise<string> {
	const { projectURL, publishableKey, serviceRoleKey } = environmentOf(platform);

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');
	if (isPersonalKey(accessToken)) error(403, 'keys are made and revoked signed in, not with a key');

	const { data: account } = await asMember({ projectURL, publishableKey }, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const claimed = await claimMemberFor(controlPlane({ projectURL, serviceRoleKey }), account.user.id, email);
	if (!claimed) error(403, 'no member here goes by that address');
	return claimed.memberID;
}

function keyStore(platform: App.Platform | undefined) {
	const { projectURL, serviceRoleKey } = environmentOf(platform);
	return controlPlane({ projectURL, serviceRoleKey });
}

export const GET: RequestHandler = async ({ request, platform }) => {
	const memberID = await memberOf(request, platform);
	return json({ keys: await personalKeys(keyStore(platform), memberID) });
};

// The key is answered once. Only its hash is kept, so nothing can read it back
// to whoever lost it; they make another by that name and it replaces this one.
export const POST: RequestHandler = async ({ request, platform }) => {
	const memberID = await memberOf(request, platform);

	const asked = (await request.json().catch(() => ({}))) as { name?: unknown; permission?: unknown };
	const name = typeof asked.name === 'string' ? asked.name.trim() : '';
	if (!name) error(400, 'a key needs a name');
	if (name.length > 64) error(400, 'that name is too long for a key');

	const permission = asked.permission === undefined ? fullPublicAPIPermission : publicAPIPermissionOf(asked.permission);
	if (!permission) error(400, 'a key reads, writes or deletes');

	return json({
		name,
		permission,
		apiKey: await issuePersonalKey(keyStore(platform), memberID, name, permission),
	});
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const memberID = await memberOf(request, platform);

	const name = (url.searchParams.get('name') ?? '').trim();
	if (!name) error(400, 'a revocation names the key');

	if (!(await forgetPersonalKey(keyStore(platform), memberID, name))) {
		error(404, 'no key of yours goes by that');
	}
	return json({ forgotten: name });
};
