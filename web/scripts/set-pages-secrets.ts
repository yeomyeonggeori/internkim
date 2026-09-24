//   monkeys run @production bun run web/scripts/set-pages-secrets.ts --project internkim

import { requiredSetting, setting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');
const projectURL = requiredSetting('SUPABASE_URL');
const publishableKey = requiredSetting('SUPABASE_PUBLISHABLE_KEY');
const secretKey = setting('SUPABASE_SECRET_KEY') || requiredSetting('SUPABASE_SERVICE_ROLE_KEY');
const signingKey = requiredSetting('SUPABASE_JWT_SIGNING_KEY');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? requiredSetting('CLOUDFLARE_ACCOUNT_ID');
const project = argument('project') ?? 'internkim';

const variables = {
	SUPABASE_URL: { type: 'secret_text', value: projectURL },
	SUPABASE_PUBLISHABLE_KEY: { type: 'secret_text', value: publishableKey },
	SUPABASE_SECRET_KEY: { type: 'secret_text', value: secretKey },
	SUPABASE_JWT_SIGNING_KEY: { type: 'secret_text', value: signingKey },
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
