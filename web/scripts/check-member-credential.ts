//   bun run web/scripts/check-member-credential.ts --url http://127.0.0.1:54321 --key <service role key>

import { createClient } from '@supabase/supabase-js';
import { keepMemberCredential, memberCredential } from '../src/lib/server/member-credential';
import {
	mailAccountCredentialKind,
	messengerIdentityCredentialKind
} from '../src/lib/server/public-api/catalog/credential';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const projectURL = argument('url') ?? process.env.SUPABASE_URL ?? '';
const serviceRoleKey = argument('key') ?? process.env.SUPABASE_SECRET_KEY ?? '';
if (!projectURL || !serviceRoleKey) throw new Error('pass --url and --key');

const admin = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

const slug = `credential-check-${crypto.randomUUID().slice(0, 8)}`;
const company = await admin
	.from('company')
	.insert({ name: 'Credential check', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' })
	.select('id')
	.single();
if (company.error) throw new Error(company.error.message);
const companyID = company.data.id as string;

async function addMember(email: string): Promise<string> {
	const member = await admin
		.from('member')
		.insert({ company_id: companyID, email, status: 'active' })
		.select('id')
		.single();
	if (member.error) throw new Error(member.error.message);
	return member.data.id as string;
}

let failed = false;
try {
	const first = await addMember(`one-${slug}@example.com`);
	const second = await addMember(`two-${slug}@example.com`);

	await keepMemberCredential(admin, first, {
		kind: messengerIdentityCredentialKind,
		externalID: `U-one-${slug}`,
		secret: 'token-for-one'
	});
	await keepMemberCredential(admin, second, {
		kind: messengerIdentityCredentialKind,
		externalID: `U-two-${slug}`,
		secret: 'token-for-two'
	});

	const forFirst = await memberCredential(admin, first, messengerIdentityCredentialKind);
	const forSecond = await memberCredential(admin, second, messengerIdentityCredentialKind);

	await keepMemberCredential(admin, first, {
		kind: messengerIdentityCredentialKind,
		externalID: `U-one-${slug}`,
		secret: 'token-for-one-rotated'
	});
	const rotated = await memberCredential(admin, first, messengerIdentityCredentialKind);

	const rows = await admin.from('credential').select('id').eq('member_id', first);

	const companySecret = await admin.rpc('write_company_secret', {
		secret_id: null,
		secret_name: `${companyID}:${messengerIdentityCredentialKind}`,
		secret_value: 'the-company-admin-password'
	});
	if (companySecret.error) throw new Error(companySecret.error.message);
	const pointed = await admin
		.from('credential')
		.update({ vault_secret_id: companySecret.data })
		.eq('member_id', second)
		.eq('kind', messengerIdentityCredentialKind);
	if (pointed.error) throw new Error(pointed.error.message);
	const afterTampering = await memberCredential(admin, second, messengerIdentityCredentialKind);

	const findings = [
		['each member reads their own secret', forFirst?.secret === 'token-for-one'],
		['and never the other one', forSecond?.secret === 'token-for-two'],
		['the external id comes back', forFirst?.externalID === `U-one-${slug}`],
		['rotating replaces rather than adds', rotated?.secret === 'token-for-one-rotated'],
		['one row per member and kind', (rows.data?.length ?? -1) === 1],
		['a member with no credential gets nothing', (await memberCredential(admin, second, mailAccountCredentialKind)) === null],
		[
			'pointing your own row at another secret gains you nothing',
			afterTampering?.secret === 'token-for-two'
		]
	] as const;

	for (const [what, held] of findings) console.log(`${held ? 'ok  ' : 'FAIL'} ${what}`);
	failed = findings.some(([, held]) => !held);
} finally {
	await admin.from('company').delete().eq('id', companyID);
}

if (failed) process.exit(1);
