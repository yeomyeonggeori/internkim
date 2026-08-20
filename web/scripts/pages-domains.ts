//   bun run web/scripts/pages-domains.ts --project <name> [--attach <hostname>]

import { requiredSetting } from './repository-setting';

const token = requiredSetting('CLOUDFLARE_API_TOKEN');

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const accountID = argument('account') ?? '694280310d0ed1189a2a54c4a546403e';
const project = argument('project');
const attaching = argument('attach');
if (!project) throw new Error('pass --project <name>');

const base = `https://api.cloudflare.com/client/v4/accounts/${accountID}/pages/projects/${project}/domains`;
const headers = { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };

if (attaching) {
	const response = await fetch(base, { method: 'POST', headers, body: JSON.stringify({ name: attaching }) });
	const body = (await response.json()) as { success: boolean; errors?: unknown; result?: { name: string; status: string } };
	if (!body.success) throw new Error(JSON.stringify(body.errors));
	console.log(`attached ${body.result?.name} — ${body.result?.status}`);
}

const listed = await fetch(base, { headers });
const body = (await listed.json()) as { success: boolean; errors?: unknown; result?: { name: string; status: string }[] };
if (!body.success) throw new Error(JSON.stringify(body.errors));
for (const entry of body.result ?? []) console.log(`  ${entry.name.padEnd(28)} ${entry.status}`);
if ((body.result ?? []).length === 0) console.log('  no domains attached');
