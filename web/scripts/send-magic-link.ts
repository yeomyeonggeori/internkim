//   bun run web/scripts/send-magic-link.ts --email you@example.com [--redirect <url>]

import { createClient } from '@supabase/supabase-js';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const email = argument('email');
const redirectTo = argument('redirect');
if (!email) throw new Error('pass --email <address>');

const projectURL = process.env.SUPABASE_URL ?? '';
const publishableKey =
	process.env.SUPABASE_PUBLISHABLE_KEY ?? process.env.VITE_SUPABASE_PUBLISHABLE_KEY ?? '';
if (!projectURL || !publishableKey) throw new Error('set SUPABASE_URL and SUPABASE_PUBLISHABLE_KEY');

const client = createClient(projectURL, publishableKey, {
	auth: { persistSession: false, autoRefreshToken: false },
});

const { error } = await client.auth.signInWithOtp({
	email,
	options: { shouldCreateUser: false, emailRedirectTo: redirectTo },
});

if (error) {
	console.log(`refused: ${error.message}`);
	process.exit(1);
}
console.log(`accepted for ${email} — check the inbox, and remember acceptance is not delivery`);
