//   bun run web/scripts/reseal-mail-accounts.ts --url http://127.0.0.1:54321 --key <service role key>
// Seals each mail password still readable in Vault to its company's box key; a company without one is named and left.

import { createClient, type SupabaseClient } from '@supabase/supabase-js';
import { connectedBoxOf } from '../src/lib/server/box';
import { keepMailAccount, mailAccountOfMember, type MailAccountOwner } from '../src/lib/server/mail-account';
import { mailAccountCredentialKind } from '../src/lib/server/public-api/catalog/credential';

export type ResealReport = {
	sealed: MailAccountOwner[];
	alreadySealed: MailAccountOwner[];
	withoutABox: MailAccountOwner[];
};

export async function resealMailAccounts(client: SupabaseClient): Promise<ResealReport> {
	const report: ResealReport = { sealed: [], alreadySealed: [], withoutABox: [] };
	for (const owner of await mailAccountOwners(client)) {
		const held = await mailAccountOfMember(client, owner.memberID);
		if (!held) continue;
		if (!held.IMAPPassword && !held.SMTPPassword) {
			report.alreadySealed.push(owner);
			continue;
		}
		if (!(await connectedBoxOf(client, owner.companyID))) {
			report.withoutABox.push(owner);
			continue;
		}
		await keepMailAccount(client, owner, held);
		report.sealed.push(owner);
	}
	return report;
}

async function mailAccountOwners(client: SupabaseClient): Promise<MailAccountOwner[]> {
	const credentials = await client
		.from('credential')
		.select('member_id')
		.eq('kind', mailAccountCredentialKind)
		.not('member_id', 'is', null)
		.returns<{ member_id: string }[]>();
	if (credentials.error) throw new Error(credentials.error.message);

	const members = await client
		.from('member')
		.select('id, company_id')
		.in('id', credentials.data.map((credential) => credential.member_id))
		.returns<{ id: string; company_id: string }[]>();
	if (members.error) throw new Error(members.error.message);
	return members.data.map((member) => ({ companyID: member.company_id, memberID: member.id }));
}

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

function described(owners: MailAccountOwner[]): string {
	return owners.map((owner) => `  company ${owner.companyID} member ${owner.memberID}`).join('\n');
}

if (import.meta.main) {
	const projectURL = argument('url') ?? '';
	const serviceRoleKey = argument('key') ?? '';
	if (!projectURL || !serviceRoleKey) throw new Error('pass --url and --key');

	const client = createClient(projectURL, serviceRoleKey, {
		auth: { autoRefreshToken: false, persistSession: false }
	});
	const report = await resealMailAccounts(client);
	console.log(`sealed ${report.sealed.length}, already sealed ${report.alreadySealed.length}`);
	if (report.withoutABox.length > 0) {
		console.log(`left readable because the company has no box key:\n${described(report.withoutABox)}`);
	}
}
