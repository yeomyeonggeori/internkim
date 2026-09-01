//   bun run web/scripts/check-notification-config.ts --url https://<project>.supabase.co

import { createClient } from '@supabase/supabase-js';
import { issueAgentKey, revokeAgent } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? process.env.SUPABASE_URL ?? '';
const serviceRoleKey = argument('key') ?? process.env.SUPABASE_SECRET_KEY ?? '';
const notifyURL = `${projectURL.replace(/\/+$/, '')}/functions/v1/notify`;
if (!projectURL || !serviceRoleKey) throw new Error('pass --url and --key');

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

const companies = await admin.from('company').select('id').returns<{ id: string }[]>();
if (companies.error) throw new Error(companies.error.message);
const company = companies.data?.[0];
if (!company) throw new Error('the record holds no company to ask about');

const agent = await issueAgentKey(admin, company.id, `notify-config-${crypto.randomUUID().slice(0, 8)}`);
let failed = false;
try {
	const response = await fetch(notifyURL, {
		method: 'POST',
		headers: { Authorization: `Bearer ${agent.apiKey}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({})
	});
	const said = await response.text();

	const reachedTheRequest = response.status === 400;
	console.log(`${response.status === 503 ? 'FAIL' : 'ok  '} ${projectURL} holds the keys a push needs`);
	console.log(`${reachedTheRequest ? 'ok  ' : 'FAIL'} and got far enough to judge the request itself`);
	console.log(`answered ${response.status}: ${said.slice(0, 160)}`);
	failed = !reachedTheRequest;
} finally {
	await revokeAgent(admin, agent.agentID);
}

if (failed) process.exit(1);
