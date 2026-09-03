import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `attendance-tools-${Date.now()}`;
const now = new Date();

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

function dayShiftedBy(days: number): string {
	return new Date(now.getTime() + days * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Attendance Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;
	await client.from('company').update({ work_locations: [{ name: '사무실' }, { name: '재택' }] }).eq('id', companyID);

	sampleID = await addMember(client, companyID, `${slug}-sample@example.test`);
	await client.from('member').update({ name: '이샘플' }).eq('id', sampleID);
	await client.from('member').update({ name: '최견본' }).eq('id', adminID);

	sample = await signedInMember(sampleID, `${slug}-sample@example.test`);
	admin = await signedInMember(adminID, `${slug}-admin@example.test`);
}, networkHookTimeout);

afterAll(async () => {
	if (!companyID) return;
	const { data: members } = await client.from('member').select('user_id').eq('company_id', companyID);
	await client.from('company').delete().eq('id', companyID);
	for (const member of members ?? []) {
		if (member.user_id) await client.auth.admin.deleteUser(member.user_id);
	}
}, networkHookTimeout);

function asSample(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(sample, client, sampleID, name, input, now);
}

function asAdmin(name: string, input: Record<string, unknown> = {}) {
	return runToolOverTheRecord(admin, client, adminID, name, input, now);
}

function resultOf(answer: { body: unknown }): Record<string, unknown> {
	return (answer.body as { result: Record<string, unknown> }).result;
}

describe('the attendance tools write a record and say when the company was told', () => {
	test('a record inside the three days is written without telling anybody', async () => {
		const added = await asSample('attendance_add', {
			kind: 'clock_in',
			date: dayShiftedBy(-1),
			time: '09:00',
			location: '사무실',
			reason: '출근 찍는 것을 잊었습니다'
		});
		expect(added.status).toBe(200);
		expect(resultOf(added).status).toBe('added');
		expect(resultOf(added).eventID).toBeString();
		expect(resultOf(added).backdated).toBe(false);
	});

	test('the list answers with what was written', async () => {
		const listed = await asSample('attendance_list', { from: dayShiftedBy(-10) });
		const attendance = resultOf(listed).attendance as { kind: string; time: string; person: string }[];
		expect(attendance).toHaveLength(1);
		expect(attendance[0].kind).toBe('clock_in');
		expect(attendance[0].time).toBe('09:00');
		expect(attendance[0].person).toBe('이샘플');
	});

	test('a record older than three days is an administrator to write, so the record asks one', async () => {
		const asked = await asSample('attendance_add', {
			kind: 'clock_in',
			date: dayShiftedBy(-30),
			time: '09:00',
			location: '재택',
			reason: '한 달 전 재택 출근 누락'
		});
		expect(asked.status).toBe(200);
		expect(resultOf(asked).status).toBe('asked');
		expect(resultOf(asked).eventID).toBeNull();

		const listed = await asSample('attendance_list', { from: dayShiftedBy(-40) });
		expect(resultOf(listed).count).toBe(1);
	});

	test('a clock with no day and no time is written at the moment it is called', async () => {
		const clocked = await asSample('attendance_add', { kind: 'clock_out' });
		expect(clocked.status).toBe(200);
		expect(resultOf(clocked).status).toBe('added');
		expect(resultOf(clocked).backdated).toBe(false);

		const removed = await asSample('attendance_delete', {
			eventHint: resultOf(clocked).eventID as string,
			reason: '방금 찍은 것을 되돌립니다'
		});
		expect(resultOf(removed).status).toBe('removed');
	});

	test('a record written by hand is taken without a reason', async () => {
		const written = await asSample('attendance_add', {
			kind: 'clock_out',
			date: dayShiftedBy(-1),
			time: '18:00'
		});
		expect(written.status).toBe(200);
		expect(resultOf(written).status).toBe('added');

		const removed = await asSample('attendance_delete', {
			eventHint: resultOf(written).eventID as string,
			reason: '손으로 넣은 기록을 되돌립니다'
		});
		expect(resultOf(removed).status).toBe('removed');
	});

	test('a hint that names nothing comes back with candidates rather than a guess', async () => {
		const missed = await asSample('attendance_update', {
			corrections: [{ eventHint: '있지도 않은 기록', time: '10:00' }],
			reason: '잘못된 힌트'
		});
		expect(missed.status).toBe(409);
		expect((missed.body as { candidates: string[] }).candidates).toEqual([]);
	});

	test('a correction inside the three days moves the record', async () => {
		const listed = await asSample('attendance_list', { from: dayShiftedBy(-10) });
		const [held] = resultOf(listed).attendance as { eventID: string }[];
		const corrected = await asSample('attendance_update', {
			corrections: [{ eventHint: held.eventID, time: '08:30' }],
			reason: '실제로는 8시 30분에 출근했습니다'
		});
		expect(resultOf(corrected).status).toBe('corrected');
		expect(resultOf(corrected).backdated).toBe(false);
		expect(resultOf(corrected).eventID).toBe(held.eventID);

		const again = await asSample('attendance_list', { from: dayShiftedBy(-10) });
		const [moved] = resultOf(again).attendance as { time: string; wasCorrected: boolean }[];
		expect(moved.time).toBe('08:30');
		expect(moved.wasCorrected).toBe(true);
	});

	test('a removal takes the record out of every read', async () => {
		const listed = await asSample('attendance_list', { from: dayShiftedBy(-10) });
		const [held] = resultOf(listed).attendance as { eventID: string }[];
		const removed = await asSample('attendance_delete', {
			eventHint: held.eventID,
			reason: '그날은 출근하지 않았습니다'
		});
		expect(resultOf(removed).status).toBe('removed');
		expect(resultOf(await asSample('attendance_list', { from: dayShiftedBy(-10) })).count).toBe(0);
	});

	test('an exact identifier resolves even outside the days a hint is scanned in', async () => {
		const added = await asAdmin('attendance_add', {
			personHint: '이샘플',
			kind: 'clock_in',
			date: dayShiftedBy(-120),
			time: '09:00',
			location: '사무실',
			reason: '넉 달 전 기록'
		});
		const eventID = resultOf(added).eventID as string;

		const corrected = await asAdmin('attendance_update', {
			corrections: [{ eventHint: eventID, time: '09:30' }],
			reason: '넉 달 전 기록의 시각 정정'
		});
		expect(corrected.status).toBe(200);
		expect(resultOf(corrected).status).toBe('corrected');
	});

	test('an administrator writes an old record for somebody else', async () => {
		const added = await asAdmin('attendance_add', {
			personHint: '이샘플',
			kind: 'clock_out',
			date: dayShiftedBy(-29),
			time: '18:00',
			reason: '관리자가 대신 기록합니다'
		});
		expect(resultOf(added).status).toBe('added');
	});
});
