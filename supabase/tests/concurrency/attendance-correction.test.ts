import { afterAll, beforeAll, describe, expect, test } from 'bun:test';
import {
	actAs,
	administratorSession,
	backendProcessID,
	createLoginRole,
	dropLoginRole,
	sessionAs,
	refusalOf,
	waitUntilWaitingOnLock,
	type LoginRole
} from './racing-sessions';

const userID = '4c000000-0000-0000-0000-000000000001';
const companyID = '4c000000-0000-0000-0000-0000000000a0';
const memberID = '4c000000-0000-0000-0000-0000000000a1';
const clockInID = '4c000000-0000-0000-0000-000000000101';
const clockOutID = '4c000000-0000-0000-0000-000000000102';

const racer: LoginRole = {
	name: 'attendance_correction_racer',
	password: crypto.randomUUID(),
	grants: ['usage on schema public', 'execute on function public.attendance_correct(jsonb, text)', 'execute on function public.attendance_change_undo(uuid)']
};

const administrator = administratorSession();

async function removeFixtures(): Promise<void> {
	await administrator`delete from public.attendance where member_id = ${memberID}`;
	await administrator`delete from public.member where id = ${memberID}`;
	await administrator`delete from public.company where id = ${companyID}`;
	await administrator`delete from auth.users where id = ${userID}`;
	await dropLoginRole(administrator, racer.name);
}

async function insertFixtures(): Promise<void> {
	await administrator`insert into auth.users (id, email) values (${userID}, 'attendance-race@example.test')`;
	await administrator`
		insert into public.company (id, name, slug, country, locale, timezone, work_locations)
		values (${companyID}, '샘플회사', 'attendance-race', 'KR', 'ko', 'Asia/Seoul', '[{"name":"Office"}]')
	`;
	await administrator`
		insert into public.member (id, company_id, email, user_id, status, is_admin)
		values (${memberID}, ${companyID}, 'attendance-race@example.test', ${userID}, 'active', true)
	`;
	await administrator`
		insert into public.attendance (id, member_id, kind, location, occurred_at) values
			(${clockInID}, ${memberID}, 'clock_in', 'Office', '2026-08-10 09:00:00+09'),
			(${clockOutID}, ${memberID}, 'clock_out', null, '2026-08-10 10:00:00+09')
	`;
}

function correction(eventID: string, localTime: string): string {
	return JSON.stringify([{ event_id: eventID, local_date: '2026-08-10', local_time: localTime, location: 'Office' }]);
}

async function recordedLocalTimes(): Promise<string[]> {
	const rows: { local_time: string }[] = await administrator`
		select to_char(occurred_at at time zone 'Asia/Seoul', 'YYYY-MM-DD HH24:MI') as local_time
		from public.attendance
		where member_id = ${memberID}
		order by id
	`;
	return rows.map((row) => row.local_time);
}

beforeAll(async () => {
	await removeFixtures();
	await createLoginRole(administrator, racer);
	await insertFixtures();
});

afterAll(async () => {
	await removeFixtures();
	await administrator.close();
});

describe('attendance correction', () => {
	test('undo locks all member events in correction order before reading its target', async () => {
		const holder = administratorSession();
		const challenger = sessionAs(racer);
		try {
			await actAs(holder, userID);
			await actAs(challenger, userID);
			await holder`select public.attendance_correct(${correction(clockInID, '09:10')}::text::jsonb, '첫 번째 수정')`;
			await holder`select public.attendance_correct(${correction(clockOutID, '10:10')}::text::jsonb, '두 번째 수정')`;
			await holder`begin`;
			await holder`select id from public.attendance where id = ${clockInID} for update`;
			const challengerProcessID = await backendProcessID(challenger);
			const challenge = challenger`select public.attendance_change_undo(${clockOutID}::uuid)`;
			const answer = Promise.resolve(challenge);
			expect(await waitUntilWaitingOnLock(administrator, challengerProcessID)).toBe(true);
			await holder`select public.attendance_change_undo(${clockInID}::uuid)`;
			await holder`commit`;
			await answer;
			expect(await recordedLocalTimes()).toEqual(['2026-08-10 09:00', '2026-08-10 10:00']);
		} finally {
			await holder`rollback`;
			await holder.close();
			await challenger.close();
		}
	});
	test('a competing request waits for the member event lock, then rechecks order after the first commit', async () => {
		const holder = sessionAs(racer);
		const challenger = sessionAs(racer);
		try {
			await actAs(holder, userID);
			await actAs(challenger, userID);
			await holder`begin`;
			await holder`select public.attendance_correct(${correction(clockInID, '09:50')}::text::jsonb, '첫 번째 수정')`;

			const challengerProcessID = await backendProcessID(challenger);
			const challengeRefusal = refusalOf(
				challenger`select public.attendance_correct(${correction(clockOutID, '09:20')}::text::jsonb, '두 번째 수정')`
			);
			const isChallengerWaiting = await waitUntilWaitingOnLock(administrator, challengerProcessID);
			await holder`commit`;

			expect(isChallengerWaiting).toBe(true);
			expect(await challengeRefusal).toBe('attendance correction cannot reorder events');
			expect(await recordedLocalTimes()).toEqual(['2026-08-10 09:50', '2026-08-10 10:00']);
		} finally {
			await holder.close();
			await challenger.close();
		}
	});
});
