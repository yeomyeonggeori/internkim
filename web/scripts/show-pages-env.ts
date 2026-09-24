//   monkeys run @production bun run web/scripts/show-pages-env.ts --project <name>

import { requiredSetting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? requiredSetting('CLOUDFLARE_ACCOUNT_ID');
const project = argument('project');
if (!project) throw new Error('pass --project <name>');

const response = await fetch(`https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}`, {
	headers: { Authorization: `Bearer ${token}` },
});
const body = (await response.json()) as {
	result?: {
		production_branch?: string;
		deployment_configs?: Record<string, { env_vars?: Record<string, { type?: string }> }>;
	};
};

console.log(`production branch: ${body.result?.production_branch ?? '(unknown)'}`);
for (const [name, config] of Object.entries(body.result?.deployment_configs ?? {})) {
	const variables = Object.entries(config.env_vars ?? {}).map(([key, value]) => `${key}:${value?.type ?? '?'}`);
	console.log(`${name.padEnd(12)} ${variables.join(', ') || '(none)'}`);
}
