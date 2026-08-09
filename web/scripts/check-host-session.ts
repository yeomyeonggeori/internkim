//   bun run web/scripts/check-host-session.ts --url http://127.0.0.1:54321 --key <service role key>

import { createClient } from '@supabase/supabase-js';
import { hostAddressOf, sessionForHost } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? process.env.SUPABASE_URL ?? '';
const serviceRoleKey = argument('key') ?? process.env.SUPABASE_SECRET_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('pass --url and --key');

const apiKey = `check-host-session-${crypto.randomUUID()}`;
const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

async function createCompanyWithAgent(): Promise<string> {
	const slug = `host-check-${crypto.randomUUID().slice(0, 8)}`;
	const company = await admin
		.from('company')
		.insert({ name: 'Host check', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' })
		.select('id')
		.single();
	if (company.error) throw new Error(company.error.message);
	const agent = await admin
		.from('agent')
		.insert({ company_id: company.data.id, name: 'check', api_key_hash: await hashOf(apiKey) });
	if (agent.error) throw new Error(agent.error.message);
	return company.data.id;
}

async function removeCompany(companyID: string): Promise<void> {
	await admin.from('company').delete().eq('id', companyID);
	const accounts = await admin.auth.admin.listUsers();
	const host = accounts.data.users.find((user) => user.email === hostAddressOf(companyID));
	if (host) await admin.auth.admin.deleteUser(host.id);
}

function claimsOf(accessToken: string): { app_metadata?: { company_id?: string } } {
	const payload = accessToken.split('.')[1] ?? '';
	return JSON.parse(atob(payload.replace(/-/g, '+').replace(/_/g, '/')));
}

const companyID = await createCompanyWithAgent();
try {
	const session = await sessionForHost({ projectURL, serviceRoleKey }, apiKey);
	const claims = claimsOf(session.accessToken);

	const asHost = createClient(projectURL, serviceRoleKey, {
		auth: { autoRefreshToken: false, persistSession: false },
		global: { headers: { Authorization: `Bearer ${session.accessToken}` } }
	});
	const seen = await asHost.rpc('my_app_company');
	const members = await admin.from('member').select('id').eq('company_id', companyID);

	const findings = [
		['the token carries the company', claims.app_metadata?.company_id === companyID],
		['my_app_company() reads it back', seen.data === companyID],
		['the host owns no member row', (members.data?.length ?? -1) === 0]
	] as const;

	for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
	if (findings.some(([, held]) => !held)) process.exit(1);
} finally {
	await removeCompany(companyID);
}
