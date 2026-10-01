//   monkeys run @production bun run web/scripts/set-pages-secrets.ts --project internkim

import declarations from '../../tools/environment.json';
import { pagesVariablesFromVault, variablesRequiredOnPages } from './pages-variables';
import { requiredSetting, setting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');
const variables = pagesVariablesFromVault(variablesRequiredOnPages(declarations), setting);

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? requiredSetting('CLOUDFLARE_ACCOUNT_ID');
const project = argument('project') ?? 'internkim';

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
console.log(`${project} now carries ${Object.keys(variables).join(', ')}`);
