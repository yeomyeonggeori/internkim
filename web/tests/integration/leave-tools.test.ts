import { afterAll, beforeAll, describe, expect, mock, test } from 'bun:test';
import { addMember, asMember, controlPlane, provisionCompany, sessionForMember } from '../../src/lib/server/control-plane';
import { projectURL, publishableKey, serviceRoleKey } from './supabase-environment';

mock.module('$env/dynamic/private', () => ({
	env: { SUPABASE_URL: projectURL, SUPABASE_SECRET_KEY: serviceRoleKey, SUPABASE_PUBLISHABLE_KEY: publishableKey }
}));

const { runToolOverTheRecord, recordRunsTheTool } = await import('../../src/lib/server/public-api/record');

const networkHookTimeout = 60_000;
const client = controlPlane({ projectURL, serviceRoleKey });
const slug = `leave-tools-${Date.now()}`;
const now = new Date('2026-08-26T03:00:00.000Z');
const grantedDays = 15;

let companyID = '';
let sampleID = '';
let adminID = '';
let sample: ReturnType<typeof asMember>;
let admin: ReturnType<typeof asMember>;

async function signedInMember(memberID: string, email: string): Promise<ReturnType<typeof asMember>> {
	const { data: account } = await client.auth.admin.createUser({ email, email_confirm: true });
	await client.from('member').update({ user_id: account.user!.id, status: 'active' }).eq('id', memberID);
	const session = await sessionForMember({ projectURL, serviceRoleKey }, memberID);
	return asMember({ projectURL, publishableKey }, session.accessToken);
}

beforeAll(async () => {
	const provisioned = await provisionCompany(
		client,
		{ name: 'Leave Tools Test', slug, country: 'KR', locale: 'ko', timezone: 'Asia/Seoul' },
		`${slug}-admin@example.test`
	);
	companyID = provisioned.companyID;
	adminID = provisioned.adminMemberID;
	await client.from('company').update({ leave_days: grantedDays }).eq('id', companyID);

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

// The reader the attendance page uses, so a row this suite writes is held to
// the meaning that page reads it with.
function leaveCoversDay(days: number, row: { starts_at: string; ends_at: string }, day: string): boolean {
	const companyDay = (value: string) =>
		new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Seoul' }).format(new Date(value));
	if (days > 0.5) return day >= companyDay(row.starts_at) && day < companyDay(row.ends_at);
	return day === companyDay(row.starts_at);
}

function resultOf(answer: { status: number; body: unknown }): Record<string, unknown> {
	expect(answer.status).toBe(200);
	return (answer.body as { result: Record<string, unknown> }).result;
}

describe('which leave tools run over the record', () => {
	test('are the four of them', () => {
		for (const name of ['leave_list', 'leave_balance', 'leave_request', 'leave_decide']) {
			expect(recordRunsTheTool(name)).toBe(true);
		}
	});
});

describe('a leave the record has not decided', () => {
	test('is filed as requested, against the kind the company registers', async () => {
		const filed = resultOf(
			await asSample('leave_request', {
				kind: '연차',
				startsAt: '2026-09-01',
				endsAt: '2026-09-02',
				days: 2,
				note: '가족 여행'
			})
		);

		expect(filed.status).toBe('requested');
		expect(filed.person).toBe('이샘플');
		expect(filed.kind).toBe('연차');
		expect(filed.days).toBe(2);
		expect(filed.isPaid).toBe(true);
		expect(filed.isDeducted).toBe(true);
		expect(filed.startDate).toBe('2026-09-01');
		expect(filed.endDate).toBe('2026-09-02');
	});

	test('covering whole days, it ends at the midnight after the last of them', async () => {
		const filed = resultOf(
			await asSample('leave_request', { kind: '연차', startsAt: '2026-06-01', endsAt: '2026-06-01', days: 1 })
		);
		expect(filed.endDate).toBe('2026-06-01');

		const { data } = await client
			.from('leave')
			.select('starts_at, ends_at')
			.eq('id', filed.leaveID as string)
			.single<{ starts_at: string; ends_at: string }>();
		expect(data!.starts_at).toBe('2026-05-31T15:00:00+00:00');
		expect(data!.ends_at).toBe('2026-06-01T15:00:00+00:00');
		expect(leaveCoversDay(1, data!, '2026-06-01')).toBe(true);
	});

	test('covering part of one day, it ends at the clock time it ends at', async () => {
		const filed = resultOf(
			await asSample('leave_request', {
				kind: '연차',
				startsAt: '2026-06-08T09:00:00+09:00',
				endsAt: '2026-06-08T14:00:00+09:00',
				days: 0.5
			})
		);
		expect(filed.endDate).toBe('2026-06-08');

		const { data } = await client
			.from('leave')
			.select('ends_at')
			.eq('id', filed.leaveID as string)
			.single<{ ends_at: string }>();
		expect(data!.ends_at).toBe('2026-06-08T05:00:00+00:00');
	});

	test('spends nothing until somebody decides it', async () => {
		const balance = resultOf(await asSample('leave_balance', { year: 2026 }));
		expect(balance.grantedDays).toBe(grantedDays);
		expect(balance.remainingDays).toBe(grantedDays);
		expect(balance.tracking).toBe('managed');
	});

	test('is refused a kind the company does not register, and told which it has', async () => {
		const answer = await asSample('leave_request', {
			kind: '안식년',
			startsAt: '2026-10-01',
			endsAt: '2026-10-01',
			days: 1
		});

		expect(answer.status).toBe(409);
		expect((answer.body as { registered: string[] }).registered).toContain('연차');
	});
});

describe('deciding a leave', () => {
	test('is refused to the person who asked for it', async () => {
		const answer = await asSample('leave_decide', { leaveHint: '이샘플 · 연차 · 2026-09-01', decision: 'approved' });
		expect(answer.status).not.toBe(200);
	});

	test('an administrator approves it, and the balance follows', async () => {
		const decided = resultOf(
			await asAdmin('leave_decide', { leaveHint: '이샘플 · 연차 · 2026-09-01', decision: 'approved' })
		);
		expect(decided.status).toBe('approved');

		const balance = resultOf(await asSample('leave_balance', { year: 2026 }));
		expect(balance.remainingDays).toBe(grantedDays - 2);
		expect(balance.usedDays).toBe(2);
	});

	test('moves a leave filed for the wrong year onto the right one', async () => {
		const filed = resultOf(
			await asSample('leave_request', { kind: '연차', startsAt: '2025-06-05', endsAt: '2025-06-05', days: 1 })
		);
		expect(filed.startDate).toBe('2025-06-05');

		const moved = resultOf(
			await asAdmin('leave_update', {
				leaveHint: filed.leaveID as string,
				startsAt: '2026-06-05',
				endsAt: '2026-06-05'
			})
		);
		expect(moved.leaveID).toBe(filed.leaveID);
		expect(moved.startDate).toBe('2026-06-05');
		expect(moved.endDate).toBe('2026-06-05');
		expect(moved.days).toBe(1);

		const listed = resultOf(await asSample('leave_list', { from: '2026-06-01', to: '2026-06-30' }));
		const leave = listed.leave as { leaveID: string }[];
		expect(leave.some((row) => row.leaveID === filed.leaveID)).toBe(true);
	});

	test('takes a leave back out of the record entirely', async () => {
		const filed = resultOf(
			await asSample('leave_request', { kind: '연차', startsAt: '2026-07-06', endsAt: '2026-07-06', days: 1 })
		);

		const taken = resultOf(await asSample('leave_delete', { leaveHint: filed.leaveID as string }));
		expect(taken.leaveID).toBe(filed.leaveID);

		const listed = resultOf(await asSample('leave_list', { from: '2026-07-01', to: '2026-07-31' }));
		const leave = listed.leave as { leaveID: string }[];
		expect(leave.some((row) => row.leaveID === filed.leaveID)).toBe(false);
	});

	test('refuses a hint two rows answer to, and names them', async () => {
		await asSample('leave_request', { kind: '연차', startsAt: '2026-11-02', endsAt: '2026-11-02', days: 1 });
		await asSample('leave_request', { kind: '연차', startsAt: '2026-11-09', endsAt: '2026-11-09', days: 1 });

		const answer = await asAdmin('leave_decide', { leaveHint: '이샘플 · 연차 · 2026-11', decision: 'approved' });
		expect(answer.status).toBe(409);
		expect((answer.body as { candidates: string[] }).candidates.length).toBe(2);
	});
});

describe('listing leave', () => {
	test('answers the requester their own, newest first', async () => {
		const listed = resultOf(await asSample('leave_list'));
		const leave = listed.leave as { startDate: string; person: string }[];
		expect(listed.scope).toBe('person');
		expect(leave.every((row) => row.person === '이샘플')).toBe(true);
		expect(leave[0].startDate).toBe('2026-11-09');
		expect(listed.registeredKinds).toContain('연차');
	});

	test('narrows to a window and to a status', async () => {
		const inSeptember = resultOf(await asSample('leave_list', { from: '2026-09-01', to: '2026-09-30' }));
		expect(inSeptember.count).toBe(1);

		const approved = resultOf(await asSample('leave_list', { status: 'approved' }));
		expect((approved.leave as { status: string }[]).every((row) => row.status === 'approved')).toBe(true);
		expect(approved.count).toBe(1);
	});
});

// The attendance screens draw a part of a day against the working hours and key
// every row by the person it belongs to, and neither a display name nor a date
// can say either.
describe('what the leave screens need out of an answer', () => {
	test('a row names the person and the kind by id and says the moments it covers', async () => {
		const filed = resultOf(
			await asSample('leave_request', {
				kind: '연차',
				startsAt: '2026-07-06T09:00:00+09:00',
				endsAt: '2026-07-06T14:00:00+09:00',
				days: 0.5
			})
		);

		expect(filed.personID).toBe(sampleID);
		expect(filed.kindID).toBeString();
		expect(filed.kindID).not.toBe('');
		expect(new Date(String(filed.startsAt)).toISOString()).toBe('2026-07-06T00:00:00.000Z');
		expect(new Date(String(filed.endsAt)).toISOString()).toBe('2026-07-06T05:00:00.000Z');

		const listed = resultOf(await asSample('leave_list', { from: '2026-07-01', to: '2026-07-31' }));
		const row = (listed.leave as Record<string, unknown>[]).find(
			(each) => each.leaveID === filed.leaveID
		);
		expect(row?.personID).toBe(sampleID);
		expect(row?.kindID).toBe(filed.kindID);
		expect(row?.startsAt).toBe(filed.startsAt);
		expect(row?.endsAt).toBe(filed.endsAt);
	});

	test('an administrator files leave for somebody else and decides it', async () => {
		const filed = resultOf(
			await asAdmin('leave_request', {
				personHint: '이샘플',
				kind: '연차',
				startsAt: '2026-07-20',
				endsAt: '2026-07-21',
				days: 2,
				note: '지난 휴가를 뒤늦게 기록합니다'
			})
		);
		expect(filed.personID).toBe(sampleID);
		expect(filed.status).toBe('requested');

		const decided = resultOf(
			await asAdmin('leave_decide', { leaveHint: filed.leaveID as string, decision: 'approved' })
		);
		expect(decided.status).toBe('approved');
		expect(decided.personID).toBe(sampleID);
	});

	test('a colleague may not file leave for anybody but themselves', async () => {
		const refused = await asSample('leave_request', {
			personHint: '최견본',
			kind: '연차',
			startsAt: '2026-07-27',
			endsAt: '2026-07-27',
			days: 1
		});
		expect(refused.status).toBeGreaterThanOrEqual(400);
	});
});
