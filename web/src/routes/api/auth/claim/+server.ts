import { env } from '$env/dynamic/private';
import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { error, json } from '@sveltejs/kit';
import { accountOfAddress, controlPlane } from '$lib/server/control-plane';
import {
	anIssuedPassword,
	isAlreadyClaimed,
	mailCanCarryTheCode,
} from '$lib/server/claim-without-mail';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the control plane is not configured');

	const body = (await request.json().catch(() => ({}))) as { email?: unknown };
	const email = typeof body.email === 'string' ? body.email.trim().toLowerCase() : '';
	if (!email.includes('@')) error(400, 'that is not an address');

	const admin = controlPlane({ projectURL, serviceRoleKey });
	const { data, error: readError } = await admin
		.from('member')
		.select('id')
		.eq('email', email)
		.maybeSingle();
	if (readError) error(500, readError.message);
	if (!data) return json({ sent: true });

	if (!mailCanCarryTheCode(environment)) {
		return json({ password: await issueTheFirstClaim(admin, email, data.id) });
	}

	const { error: sendError } = await createClient(projectURL, publishableKey).auth.signInWithOtp({
		email,
		options: { shouldCreateUser: true, emailRedirectTo: `${url.origin}/auth/claim` },
	});
	if (sendError) error(429, sendError.message);
	return json({ sent: true });
};

async function issueTheFirstClaim(
	admin: SupabaseClient,
	email: string,
	memberID: string,
): Promise<string> {
	const account = (await accountOfAddress(admin, email)) ?? undefined;
	if (isAlreadyClaimed(account)) error(409, 'that sign-in is already set up');

	const password = anIssuedPassword();
	if (!account) {
		const created = await admin.auth.admin.createUser({ email, password, email_confirm: true });
		if (created.error) error(500, created.error.message);
		const linked = await admin.from('member').update({ user_id: created.data.user.id }).eq('id', memberID);
		if (linked.error) error(500, linked.error.message);
		return password;
	}

	const updated = await admin.auth.admin.updateUserById(account.id, { password });
	if (updated.error) error(500, updated.error.message);
	return password;
}
