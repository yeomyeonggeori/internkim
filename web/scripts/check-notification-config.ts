//   bun run web/scripts/check-notification-config.ts --url https://<project>.supabase.co --key <service role> --signing-key <record signing JWK>

import { createClient } from '@supabase/supabase-js';
import { hostAddressOf } from '../src/lib/server/control-plane';
import { claimABoxWithItsSession } from '../tests/support/a-claimed-box';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? process.env.SUPABASE_URL ?? '';
const serviceRoleKey = argument('key') ?? process.env.SUPABASE_SECRET_KEY ?? '';
const signingKey = argument('signing-key') ?? process.env.SUPABASE_JWT_SIGNING_KEY ?? '';
const notifyURL = `${projectURL.replace(/\/+$/, '')}/functions/v1/notify`;
if (!projectURL || !serviceRoleKey || !signingKey) throw new Error('pass --url, --key and --signing-key');

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

const slug = `notify-config-${crypto.randomUUID().slice(0, 8)}`;
const company = await admin
	.from('company')
	.insert({ name: 'Notify config check', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' })
	.select('id')
	.single<{ id: string }>();
if (company.error) throw new Error(company.error.message);

let failed = false;
try {
	const box = await claimABoxWithItsSession({ projectURL, serviceRoleKey, signingKey }, company.data.id);
	const response = await fetch(notifyURL, {
		method: 'POST',
		headers: { Authorization: `Bearer ${box.session.accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({})
	});
	const said = await response.text();

	const reachedTheRequest = response.status === 400;
	console.log(`${response.status === 503 ? 'FAIL' : 'ok  '} ${projectURL} holds the keys a push needs`);
	console.log(`${reachedTheRequest ? 'ok  ' : 'FAIL'} and got far enough to judge the request itself`);
	console.log(`answered ${response.status}: ${said.slice(0, 160)}`);
	failed = !reachedTheRequest;
} finally {
	await admin.from('company').delete().eq('id', company.data.id);
	const accounts = await admin.auth.admin.listUsers();
	const host = accounts.data.users.find((user) => user.email === hostAddressOf(company.data.id));
	if (host) await admin.auth.admin.deleteUser(host.id);
}

if (failed) process.exit(1);
