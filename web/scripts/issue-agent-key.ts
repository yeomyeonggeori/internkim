//   bun run web/scripts/issue-agent-key.ts --company <uuid> --name <what to call it>

import { controlPlane, issueAgentKey } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const name = argument('name') ?? 'first';
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const credential = await issueAgentKey(client, companyID, name);

// A provisioning script reads this back, so --quiet answers the key alone.
if (process.argv.includes('--quiet')) {
	console.log(credential.apiKey);
} else {
	console.log('\nPut this in the host environment as INTERNKIM_AGENT_KEY.');
	console.log('It is shown once. Issuing another does not stop this one — retire it when the new agent is up.\n');
	console.log(`  ${credential.apiKey}\n`);
}
