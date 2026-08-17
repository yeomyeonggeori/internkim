//   bun run web/scripts/set-pages-secrets.ts --project internkim

const token = process.env.CLOUDFLARE_API_TOKEN ?? '';
const projectURL = process.env.SUPABASE_URL ?? '';
const publishableKey = process.env.SUPABASE_PUBLISHABLE_KEY ?? '';
const secretKey = process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
if (!token) throw new Error('set CLOUDFLARE_API_TOKEN');
if (!projectURL || !publishableKey || !secretKey) throw new Error('set SUPABASE_URL, SUPABASE_PUBLISHABLE_KEY and SUPABASE_SECRET_KEY');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? '694280310d0ed1189a2a54c4a546403e';
const project = argument('project') ?? 'internkim';

const variables = {
	SUPABASE_URL: { type: 'secret_text', value: projectURL },
	SUPABASE_PUBLISHABLE_KEY: { type: 'secret_text', value: publishableKey },
	SUPABASE_SECRET_KEY: { type: 'secret_text', value: secretKey },
	CF_PAGES_PROJECT: { type: 'plain_text', value: project },
};

const response = await fetch(`https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}`, {
	method: 'PATCH',
	headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' },
	body: JSON.stringify({
		deployment_configs: {
			production: { env_vars: variables },
			preview: { env_vars: variables },
		},
	}),
});

const body = (await response.json()) as { success: boolean; errors?: unknown };
if (!body.success) throw new Error(JSON.stringify(body.errors));
console.log(`${project} now carries the central plane credentials`);
