//   bun run web/scripts/clear-tasks.ts --company <uuid>

import { controlPlane } from '../src/lib/server/control-plane';

const index = process.argv.indexOf('--company');
const companyID = index >= 0 ? process.argv[index + 1] : undefined;
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { error } = await client.from('task').delete().eq('company_id', companyID);
if (error) throw new Error(error.message);

const { count } = await client.from('task').select('*', { count: 'exact', head: true });
console.log(`tasks remaining: ${count ?? 0}`);
