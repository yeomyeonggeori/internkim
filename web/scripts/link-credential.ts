//   bun run web/scripts/link-credential.ts --email <address> --kind buzz-secret --external-id <id>

import { controlPlane, linkCredential } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const email = argument('email');
const kind = argument('kind');
const externalID = argument('external-id');
if (!email || !kind || !externalID) throw new Error('pass --email <address> --kind <kind> --external-id <id>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const member = await client.from('member').select('id, name').eq('email', email).single<{ id: string; name: string }>();
if (member.error) throw new Error(member.error.message);

await linkCredential(client, member.data.id, kind, externalID);
console.log(`${member.data.name || email} is ${kind} ${externalID}`);
