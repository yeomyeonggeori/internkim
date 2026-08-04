//   bun run web/scripts/import-org-chart.ts --sqlite <copy> --company <uuid> [--apply]

import { Database } from 'bun:sqlite';
import { controlPlane } from '../src/lib/server/control-plane';

type DeviceGroup = { id: string; name: string; parent_id: string | null; position: number };
type DeviceProfile = {
	user_id: string;
	email: string;
	job_title: string | null;
	phone_number: string | null;
	group_id: string | null;
	supervisor_id: string | null;
	hire_date: string | null;
};

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const sqlitePath = argument('sqlite');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!sqlitePath || !companyID) throw new Error('pass --sqlite <path> --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const device = new Database(sqlitePath);
const groups = device
	.query('select id, name, parent_id, position from organization_groups order by position')
	.all() as DeviceGroup[];
const profiles = device
	.query('select user_id, email, job_title, phone_number, group_id, supervisor_id, hire_date from organization_profiles')
	.all() as DeviceProfile[];

const { data: members, error: memberError } = await client
	.from('member')
	.select('id, email')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((members ?? []).map((member) => [member.email ?? '', member.id]));
const emailByDeviceUser = new Map(profiles.map((profile) => [profile.user_id, profile.email]));

console.log(`teams ${groups.length}, profiles ${profiles.length}`);
for (const group of groups) {
	const parent = groups.find((candidate) => candidate.id === (group.parent_id || ''));
	console.log(`  team ${group.name}${parent ? ` under ${parent.name}` : ''} (position ${group.position})`);
}
for (const profile of profiles) {
	const team = groups.find((group) => group.id === (profile.group_id || ''));
	const supervisorEmail = emailByDeviceUser.get(profile.supervisor_id || '');
	const known = memberByEmail.has(profile.email);
	console.log(
		`  ${profile.email.padEnd(24)} ${(profile.job_title ?? '-').padEnd(12)} team=${team?.name ?? '-'} supervisor=${supervisorEmail ?? '-'}` +
			` joined=${profile.hire_date || '-'}${known ? '' : '   (no member here, skipped)'}`,
	);
}

if (!shouldApply) {
	console.log('\ndry run — pass --apply to write');
	process.exit(0);
}

const teamIDByDeviceGroup = new Map<string, string>();
for (const group of groups) {
	const { data, error } = await client
		.from('team')
		.upsert({ company_id: companyID, name: group.name, position: group.position }, { onConflict: 'company_id,name' })
		.select('id')
		.single();
	if (error) throw new Error(`team ${group.name}: ${error.message}`);
	teamIDByDeviceGroup.set(group.id, data.id);
}

for (const group of groups) {
	if (!group.parent_id) continue;
	const parentTeamID = teamIDByDeviceGroup.get(group.parent_id);
	if (!parentTeamID) continue;
	const { error } = await client
		.from('team')
		.update({ parent_team_id: parentTeamID })
		.eq('id', teamIDByDeviceGroup.get(group.id));
	if (error) throw new Error(`team parent ${group.name}: ${error.message}`);
}
console.log(`wrote ${groups.length} teams`);

let updated = 0;
for (const profile of profiles) {
	const memberID = memberByEmail.get(profile.email);
	if (!memberID) continue;
	const supervisorEmail = emailByDeviceUser.get(profile.supervisor_id || '');
	const supervisorID = supervisorEmail ? memberByEmail.get(supervisorEmail) : undefined;
	const { error } = await client
		.from('member')
		.update({
			job_title: (profile.job_title ?? '').trim() || null,
			phone_number: (profile.phone_number ?? '').trim() || null,
			team_id: teamIDByDeviceGroup.get(profile.group_id || '') ?? null,
			supervisor_id: supervisorID && supervisorID !== memberID ? supervisorID : null,
			joined_at: profile.hire_date ? `${profile.hire_date}T00:00:00+09:00` : null,
		})
		.eq('id', memberID);
	if (error) throw new Error(`member ${profile.email}: ${error.message}`);
	updated += 1;
}
console.log(`updated ${updated} members`);
