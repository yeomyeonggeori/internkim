//   bun run web/scripts/import-org-chart.ts --file <people.json> --company <uuid> [--apply]

import { controlPlane } from '../src/lib/server/control-plane';

type DeviceGroup = { id: string; name: string; parentID?: string };
type DeviceRecord = {
	memberID: string;
	name?: string;
	email: string;
	hireDate?: string;
	jobTitle?: string;
	groupID?: string;
	phoneNumber?: string;
	supervisorID?: string;
};

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

const file = argument('file');
const companyID = argument('company');
const shouldApply = process.argv.includes('--apply');
if (!file || !companyID) throw new Error('pass --file <people.json> --company <uuid>');

const client = controlPlane({
	projectURL: process.env.SUPABASE_URL ?? '',
	serviceRoleKey: process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '',
});

const document = JSON.parse(await Bun.file(file).text()) as {
	records?: DeviceRecord[];
	availableGroups?: DeviceGroup[];
};
const records = document.records ?? [];
const groups = document.availableGroups ?? [];

const { data: members, error: memberError } = await client
	.from('member')
	.select('id, email')
	.eq('company_id', companyID);
if (memberError) throw new Error(memberError.message);
const memberByEmail = new Map((members ?? []).map((member) => [member.email ?? '', member.id]));
const memberByDeviceUser = new Map(
	records
		.filter((record) => memberByEmail.has(record.email))
		.map((record) => [record.memberID, memberByEmail.get(record.email)!])
);

const { data: teams, error: teamError } = await client.from('team').select('id, name').eq('company_id', companyID);
if (teamError) throw new Error(teamError.message);
const teamByName = new Map((teams ?? []).map((team) => [team.name, team.id]));

const teamByDeviceGroup = new Map<string, string>();
let teamsWritten = 0;
for (const [position, group] of groups.entries()) {
	let teamID = teamByName.get(group.name);
	if (!teamID) {
		if (!shouldApply) {
			teamsWritten += 1;
			continue;
		}
		const { data, error } = await client
			.from('team')
			.insert({ company_id: companyID, name: group.name, position })
			.select('id')
			.single();
		if (error) throw new Error(`team ${group.name}: ${error.message}`);
		teamID = data.id;
		teamByName.set(group.name, teamID);
		teamsWritten += 1;
	}
	teamByDeviceGroup.set(group.id, teamID);
}

if (shouldApply) {
	for (const group of groups) {
		if (!group.parentID) continue;
		const teamID = teamByDeviceGroup.get(group.id);
		const parentID = teamByDeviceGroup.get(group.parentID);
		if (!teamID || !parentID) continue;
		const { error } = await client.from('team').update({ parent_team_id: parentID }).eq('id', teamID);
		if (error) throw new Error(`team ${group.name} parent: ${error.message}`);
	}
}

let peopleWritten = 0;
const unknownPeople = new Set<string>();
for (const record of records) {
	const memberID = memberByEmail.get(record.email);
	if (!memberID) {
		unknownPeople.add(record.email);
		continue;
	}
	if (!shouldApply) {
		peopleWritten += 1;
		continue;
	}
	const { error } = await client
		.from('member')
		.update({
			name: record.name ?? null,
			job_title: record.jobTitle ?? null,
			phone_number: record.phoneNumber ?? null,
			joined_at: record.hireDate ? `${record.hireDate}T00:00:00+09:00` : null,
			team_id: record.groupID ? (teamByDeviceGroup.get(record.groupID) ?? null) : null,
			supervisor_id: record.supervisorID ? (memberByDeviceUser.get(record.supervisorID) ?? null) : null,
		})
		.eq('id', memberID);
	if (error) throw new Error(`${record.email}: ${error.message}`);
	peopleWritten += 1;
}

console.log(`${shouldApply ? 'wrote' : 'would write'}: ${teamsWritten} teams, ${peopleWritten} people`);
if (unknownPeople.size) console.log(`addresses that match no member: ${[...unknownPeople].join(', ')}`);
