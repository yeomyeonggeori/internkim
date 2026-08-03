// Gives every clock-in a location. Imported history kept the device's blanks, but a
// blank would later be read against whatever the work-location list has become, so
// the first registered location is written in now — the same value the insert
// trigger gives a live clock-in.
//   bun run web/scripts/backfill-locations.ts --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!companyID) throw new Error('pass --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const { data: company, error: companyError } = await client
	.from('company')
	.select('work_locations')
	.eq('id', companyID)
	.single();
if (companyError) throw new Error(companyError.message);

const firstLocation = (company.work_locations ?? [])[0];
if (!firstLocation) throw new Error('the company has no registered work locations');

const { data: members } = await client.from('member').select('id').eq('company_id', companyID);
const memberIDs = (members ?? []).map((member) => member.id);

const { data: blanks, error: blankError } = await client
	.from('attendance')
	.select('id')
	.in('member_id', memberIDs)
	.eq('kind', 'clock_in')
	.is('location', null);
if (blankError) throw new Error(blankError.message);

console.log(`clock-ins without a location: ${blanks?.length ?? 0} — filling with ${firstLocation}`);
if (!shouldApply) {
	console.log('dry run — pass --apply to write');
	process.exit(0);
}

const { error } = await client
	.from('attendance')
	.update({ location: firstLocation })
	.in('id', (blanks ?? []).map((row) => row.id));
if (error) throw new Error(error.message);
console.log(`filled ${blanks?.length ?? 0} clock-ins`);
