//   bun run web/scripts/link-messenger-identities.ts [--apply]

import { controlPlane, linkCredential } from '../src/lib/server/control-plane';
import { messengerIdentityCredentialKind } from '../src/lib/server/public-api/catalog/credential';

const shouldApply = process.argv.includes('--apply');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? '',
});

const members = await client
	.from('member')
	.select('id, name, messenger')
	.returns<{ id: string; name: string | null; messenger: Record<string, string> | null }[]>();
if (members.error) throw new Error(members.error.message);

const accounts = members.data.flatMap((member) =>
	Object.entries(member.messenger ?? {})
		.filter(([, externalID]) => Boolean(externalID))
		.map(([platform, externalID]) => ({ platform, externalID, name: member.name ?? '', memberID: member.id })),
);

const existing = await client
	.from('credential')
	.select('member_id, kind, external_id')
	.not('member_id', 'is', null)
	.returns<{ member_id: string; kind: string; external_id: string | null }[]>();
if (existing.error) throw new Error(existing.error.message);
const linked = new Set(existing.data.map((row) => `${row.member_id}|${row.kind}`));

let written = 0;
let already = 0;
for (const account of accounts) {
	if (linked.has(`${account.memberID}|${messengerIdentityCredentialKind}`)) {
		already += 1;
		continue;
	}
	if (shouldApply) await linkCredential(client, account.memberID, messengerIdentityCredentialKind, account.externalID);
	written += 1;
	console.log(`  ${account.name} → ${account.platform} ${account.externalID}`);
}

console.log(`${shouldApply ? 'linked' : 'would link'}: ${written}, already linked: ${already}`);
