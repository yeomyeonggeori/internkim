import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';
import { capabilityToolResultSchema } from '../../src/lib/server/public-api/catalog/tools';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `people-tools-${Date.now()}`;
const now = new Date('2026-09-03T03:00:00.000Z');

let companyID = '';
let adminID = '';
let sampleID = '';
let exampleID = '';
let admin: ReturnType<typeof asMember>;
let sample: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'People Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;

	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	exampleID = await addMember(client, companyID, `${slug}-example@example.test`);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '박예시' }).eq('id', exampleID);

	admin = await signedInMember(adminID, `${slug}-admin@example.test`);
	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(admin, client, adminID, name, input, now);
}

function asSample(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(sample, client, sampleID, name, input, now);
}

function resultOf(answer: { status: number; body: unknown }): Record<string, unknown> {
	expect(answer.status).toBe(200);
	return (answer.body as { result: Record<string, unknown> }).result;
}

function refusalOf(answer: { status: number; body: unknown }): { status: number; error: string; errorCode?: string } {
	const body = answer.body as { error: string; errorCode?: string };
	return { status: answer.status, error: body.error, errorCode: body.errorCode };
}

function holdToTheContract(name: string, result: unknown): void {
	const schema = capabilityToolResultSchema(name);
	if (!schema) throw new Error(`${name} publishes no result contract to hold its answer to`);
	const parsed = schema.safeParse(result);
	const refused = parsed.success
		? []
		: parsed.error.issues.map((issue) => `${['result', ...issue.path.map(String)].join('.')}: ${issue.message}`);
	expect({ tool: name, refused }).toEqual({ tool: name, refused: [] });
}

describe('the organization chart written one organization at a time', () => {
	test('adds, nests, renames and lists', async () => {
		const made = resultOf(await asAdmin('team_add', { name: '개발팀' }));
		holdToTheContract('team_add', made);
		expect(made.name).toBe('개발팀');
		expect(made.parentTeamID).toBe('');

		const child = resultOf(await asAdmin('team_add', { name: '플랫폼', parentHint: '개발팀', position: 1 }));
		expect(child.parentTeamID).toBe(made.teamID);
		expect(child.parentTeamName).toBe('개발팀');
		expect(child.position).toBe(1);

		const renamed = resultOf(await asAdmin('team_update', { teamHint: '플랫폼', name: '플랫폼실' }));
		expect(renamed.name).toBe('플랫폼실');
		expect(renamed.parentTeamID).toBe(made.teamID);

		const listed = resultOf(await asAdmin('team_list'));
		holdToTheContract('team_list', listed);
		expect((listed.teams as { name: string }[]).map((team) => team.name)).toEqual(['개발팀', '플랫폼실']);
	}, networkHookTimeout);

	test('asks which organization when a name reaches more than one', async () => {
		await asAdmin('team_add', { name: '영업 1팀' });
		await asAdmin('team_add', { name: '영업 2팀' });
		const refusal = refusalOf(await asAdmin('team_update', { teamHint: '영업', name: '영업본부' }));
		expect(refusal.status).toBe(409);
		expect(refusal.errorCode).toBe('interaction_required');

		await asAdmin('team_delete', { teamHint: '영업 1팀' });
		await asAdmin('team_delete', { teamHint: '영업 2팀' });
	}, networkHookTimeout);

	test('leaves the people of a removed organization with no organization', async () => {
		await asAdmin('person_update', { personHint: '박예시', teamHint: '플랫폼실' });
		const removed = resultOf(await asAdmin('team_delete', { teamHint: '플랫폼실' }));
		holdToTheContract('team_delete', removed);
		expect(removed.deleted).toBe(true);
		expect(removed.peopleLeftWithNoOrganization).toBe(1);

		const listed = resultOf(await asAdmin('person_list'));
		const example = (listed.people as { personID: string; teamID?: string }[]).find(
			(person) => person.personID === exampleID
		);
		expect(example?.teamID).toBeUndefined();
	}, networkHookTimeout);

	test('refuses to remove an organization that has organizations under it', async () => {
		await asAdmin('team_add', { name: '연구소', parentHint: '개발팀' });
		const refusal = refusalOf(await asAdmin('team_delete', { teamHint: '개발팀' }));
		expect(refusal.status).toBe(422);
		expect(refusal.error).toContain('organizations under it');
		await asAdmin('team_delete', { teamHint: '연구소' });
	}, networkHookTimeout);

	test('refuses an organization written by somebody who does not administer', async () => {
		const refusal = refusalOf(await asSample('team_add', { name: '몰래 만든 팀' }));
		expect(refusal.status).toBe(403);
	}, networkHookTimeout);
});

describe('a directory entry written through the two homes it lives in', () => {
	test('writes the account fields and the organization fields in one call', async () => {
		const written = resultOf(
			await asAdmin('person_update', {
				personHint: '이샘플',
				name: '이샘플',
				jobTitle: '편집장',
				teamHint: '개발팀',
				supervisorHint: '최견본',
				phoneNumber: '010-0000-0000',
				hireDate: '2026-03-12'
			})
		);
		holdToTheContract('person_update', written);
		expect(written.personID).toBe(sampleID);
		expect(written.jobTitle).toBe('편집장');
		expect(written.teamName).toBe('개발팀');
		expect(written.supervisorName).toBe('최견본');
		expect(written.phoneNumber).toBe('010-0000-0000');
		expect(written.hireDate).toBe('2026-03-12');
		expect(written.isAdmin).toBe(false);
	}, networkHookTimeout);

	test('leaves the fields the call did not name alone, and clears the ones it emptied', async () => {
		const written = resultOf(await asAdmin('person_update', { personHint: '이샘플', phoneNumber: '' }));
		expect(written.phoneNumber).toBeUndefined();
		expect(written.jobTitle).toBe('편집장');
		expect(written.teamName).toBe('개발팀');
	}, networkHookTimeout);

	test('answers person_list with what both homes hold', async () => {
		const listed = resultOf(await asSample('person_list'));
		holdToTheContract('person_list', listed);
		const written = (listed.people as { personID: string; jobTitle?: string; teamName?: string; handle?: string }[]).find(
			(person) => person.personID === sampleID
		);
		expect(written?.jobTitle).toBe('편집장');
		expect(written?.teamName).toBe('개발팀');
		expect(written?.handle).toBe(`@${slug}-sample`);
	}, networkHookTimeout);

	test('lets a person correct their own phone number and start date', async () => {
		const written = resultOf(
			await asSample('person_update', { personHint: sampleID, phoneNumber: '010-1111-2222', hireDate: '2026-04-01' })
		);
		expect(written.phoneNumber).toBe('010-1111-2222');
		expect(written.hireDate).toBe('2026-04-01');
	}, networkHookTimeout);

	test('refuses a field of their own that is not theirs to write, and names it', async () => {
		const refusal = refusalOf(await asSample('person_update', { personHint: sampleID, jobTitle: '대표' }));
		expect(refusal.status).toBe(403);
		expect(refusal.error).toContain('jobTitle');
	}, networkHookTimeout);

	test("refuses a colleague's organization fields, and names them", async () => {
		const refusal = refusalOf(await asSample('person_update', { personHint: '박예시', phoneNumber: '010-3333-4444' }));
		expect(refusal.status).toBe(403);
		expect(refusal.error).toContain('phoneNumber');
	}, networkHookTimeout);

	test('refuses somebody raising themselves to administer the company', async () => {
		const refusal = refusalOf(await asSample('person_update', { personHint: sampleID, isAdmin: true }));
		expect(refusal.status).toBe(403);
		expect(refusal.error).toContain('isAdmin');
	}, networkHookTimeout);

	test('asks which person when a name reaches more than one', async () => {
		const doubled = await addMember(client, companyID, `${slug}-twin@example.test`);
		await client.from('member').update({ name: '이샘플' }).eq('id', doubled);

		const refusal = refusalOf(await asAdmin('person_update', { personHint: '이샘플', jobTitle: '편집장' }));
		expect(refusal.status).toBe(409);
		expect(refusal.errorCode).toBe('interaction_required');

		await client.from('member').delete().eq('id', doubled);
	}, networkHookTimeout);

	test('asks who was meant when no name is near', async () => {
		const refusal = refusalOf(await asAdmin('person_update', { personHint: '없는사람', jobTitle: '편집장' }));
		expect(refusal.status).toBe(409);
		expect(refusal.errorCode).toBe('person_not_found');
	}, networkHookTimeout);
});

describe('inviting somebody through the plane s own invite path', () => {
	test('creates the account, the sign-in, and the directory entry', async () => {
		const invited = resultOf(
			await asAdmin('person_invite', {
				email: `${slug}-new@example.test`,
				name: '새사람',
				jobTitle: '인턴',
				teamHint: '개발팀'
			})
		);
		holdToTheContract('person_invite', invited);
		expect(invited.email).toBe(`${slug}-new@example.test`);
		expect(invited.employmentStatus).toBe('invited');
		expect(String(invited.temporaryPassword).length).toBeGreaterThan(0);

		const { data: account } = await client.auth.admin.listUsers();
		expect(account.users.some((user) => user.email === `${slug}-new@example.test`)).toBe(true);

		const listed = resultOf(await asAdmin('person_list'));
		const written = (listed.people as { personID: string; jobTitle?: string; teamName?: string }[]).find(
			(person) => person.personID === invited.personID
		);
		expect(written?.jobTitle).toBe('인턴');
		expect(written?.teamName).toBe('개발팀');
	}, networkHookTimeout);

	test('refuses an invitation from somebody who does not administer', async () => {
		const refusal = refusalOf(
			await asSample('person_invite', { email: `${slug}-refused@example.test`, name: '거절된사람' })
		);
		expect(refusal.status).toBe(403);
		expect(refusal.errorCode).toBe('administrator_only');

		const { data: held } = await client
			.from('member')
			.select('id')
			.eq('email', `${slug}-refused@example.test`)
			.maybeSingle();
		expect(held).toBeNull();
	}, networkHookTimeout);
});
