import { env } from '$env/dynamic/private';
import {
	asMember,
	claimMemberFor,
	controlPlane,
	issueMemberKey,
	memberKeys,
	revokeMemberKey,
} from '$lib/server/control-plane';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

type Caller = { memberID: string; companyID: string };

async function callerOf(request: Request, platform: App.Platform | undefined): Promise<Caller> {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');

	const { data: account } = await asMember({ projectURL, publishableKey }, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const claimed = await claimMemberFor(controlPlane({ projectURL, serviceRoleKey }), account.user.id, email);
	if (!claimed) error(403, 'no member here goes by that address');
	return { memberID: claimed.memberID, companyID: claimed.companyID };
}

function serviceClient(platform: App.Platform | undefined) {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	return controlPlane({ projectURL, serviceRoleKey });
}

export const GET: RequestHandler = async ({ request, platform }) => {
	const caller = await callerOf(request, platform);
	return json({ keys: await memberKeys(serviceClient(platform), caller.memberID) });
};

// A key is shown once. It is kept as a hash, so nobody - not this app, not an
// administrator - can read it back to whoever lost it.
export const POST: RequestHandler = async ({ request, platform }) => {
	const caller = await callerOf(request, platform);

	const asked = (await request.json().catch(() => ({}))) as { name?: unknown };
	const name = typeof asked.name === 'string' ? asked.name.trim() : '';
	if (!name) error(400, 'a key needs a name');
	if (name.length > 64) error(400, 'that name is too long for a key');

	const issued = await issueMemberKey(serviceClient(platform), caller.companyID, caller.memberID, name);
	return json({ key: issued.key, apiKey: issued.apiKey });
};

export const DELETE: RequestHandler = async ({ request, url, platform }) => {
	const caller = await callerOf(request, platform);

	const keyID = (url.searchParams.get('keyID') ?? '').trim();
	if (!keyID) error(400, 'a revocation names the key');

	const revoked = await revokeMemberKey(serviceClient(platform), caller.memberID, keyID);
	if (!revoked) error(404, 'no key of yours goes by that');
	return json({ revoked: keyID });
};
