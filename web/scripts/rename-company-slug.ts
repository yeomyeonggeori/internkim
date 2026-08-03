// Changes the name a company is reached at.
//   bun run web/scripts/rename-company-slug.ts --company <uuid> --slug <new>

import { controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const slug = argument('slug');
if (!companyID || !slug) throw new Error('pass --company <uuid> --slug <new>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data, error } = await client
	.from('company')
	.update({ slug })
	.eq('id', companyID)
	.select('name, slug')
	.single();
if (error) throw new Error(error.message);

console.log(`${data.name} is now reached at ${data.slug}`);
