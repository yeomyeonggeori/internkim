import { env } from '$env/dynamic/private';
import { createClient } from '@supabase/supabase-js';
import { error, json } from '@sveltejs/kit';
import { controlPlane } from '$lib/server/control-plane';
import type { RequestHandler } from './$types';

export const POST: RequestHandler = async ({ request, platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
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

	const { error: sendError } = await createClient(projectURL, publishableKey).auth.signInWithOtp({
		email,
		options: { shouldCreateUser: true, emailRedirectTo: `${url.origin}/auth/claim` },
	});
	if (sendError) error(429, sendError.message);
	return json({ sent: true });
};
