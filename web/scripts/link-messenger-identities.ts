//   bun run web/scripts/link-messenger-identities.ts [--apply]

import { controlPlane, linkCredential } from '../src/lib/server/control-plane';

const shouldApply = process.argv.includes('--apply');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const contacts = await client
	.from('contact')
	.select('platform, external_id, name, member_id')
	.not('member_id', 'is', null)
	.returns<{ platform: string; external_id: string; name: string; member_id: string }[]>();
if (contacts.error) throw new Error(contacts.error.message);

const existing = await client
	.from('credential')
	.select('member_id, kind, external_id')
	.not('member_id', 'is', null)
	.returns<{ member_id: string; kind: string; external_id: string | null }[]>();
if (existing.error) throw new Error(existing.error.message);
const linked = new Set(existing.data.map((row) => `${row.member_id}|${row.kind}`));

let written = 0;
let already = 0;
for (const contact of contacts.data) {
	if (linked.has(`${contact.member_id}|${contact.platform}`)) {
		already += 1;
		continue;
	}
	if (shouldApply) await linkCredential(client, contact.member_id, contact.platform, contact.external_id);
	written += 1;
	console.log(`  ${contact.name} → ${contact.platform} ${contact.external_id}`);
}

console.log(`${shouldApply ? 'linked' : 'would link'}: ${written}, already linked: ${already}`);
