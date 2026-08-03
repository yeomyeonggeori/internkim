// Issues the secret a host proves itself with. Shown once — only its hash is kept.
//   bun run web/scripts/issue-host-secret.ts --company <uuid>

import { controlPlane, issueHostSecret } from '../src/lib/server/control-plane';

const index = process.argv.indexOf('--company');
const companyID = index >= 0 ? process.argv[index + 1] : undefined;
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const credential = await issueHostSecret(client, companyID);
console.log('\nPut this in the host environment as INTERNKIM_HOST_SECRET.');
console.log('It is shown once; issuing again replaces it and stops the old host.\n');
console.log(`  ${credential.secret}\n`);
