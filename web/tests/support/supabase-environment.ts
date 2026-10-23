function credential(name: string): string {
	return (process.env[name] ?? '').trim();
}

export const projectURL = credential('SUPABASE_URL');
export const serviceRoleKey = credential('SUPABASE_SECRET_KEY');
export const publishableKey = credential('SUPABASE_PUBLISHABLE_KEY');
export const signingKey = credential('SUPABASE_JWT_SIGNING_KEY');

const missing = Object.entries({
	SUPABASE_URL: projectURL,
	SUPABASE_SECRET_KEY: serviceRoleKey,
	SUPABASE_PUBLISHABLE_KEY: publishableKey,
	SUPABASE_JWT_SIGNING_KEY: signingKey
})
	.filter(([, value]) => !value)
	.map(([name]) => name);

if (missing.length > 0) {
	throw new Error(
		`a test against the local plane talks to a real Supabase stack and ${missing.join(', ')} ${missing.length === 1 ? 'is' : 'are'} not set. Run it through "scripts/test-against-local-plane.sh", which reads them from the local stack, or export them yourself. Skipping would report a pass for work nobody checked.`
	);
}
