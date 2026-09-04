import { beforeEach, describe, expect, mock, test } from 'bun:test';
import { defaultLeavePolicy } from '../../../src/lib/attendance/leave-policy-defaults';

const databaseServerTime = '2026-08-18T03:04:05.678Z';

type AnsweredAttendance = {
	eventID: string;
	personID: string;
	person: string;
	kind: 'clock_in' | 'clock_out';
	date: string;
	time: string;
	occurredAt: string;
	location: string | null;
	wasCorrected: boolean;
	originalDate: string | null;
	originalTime: string | null;
	originalOccurredAt: string | null;
	reason: string | null;
};

type AnsweredLeave = {
	leaveID: string;
	personID: string;
	person: string;
	kindID: string;
	kind: string;
	days: number;
	status: string;
	isPaid: boolean;
	isDeducted: boolean;
	startDate: string;
	endDate: string;
	startsAt: string;
	endsAt: string;
	note: string | null;
};

let leaveOfTheCompany: AnsweredLeave[] = [];
let leaveCoveringTheMoment: AnsweredLeave[] = [];
let attendanceOfTheCompany: AnsweredAttendance[] = [];
const asked: { name: string; input: Record<string, unknown> }[] = [];

function answeredLeave(leave: Partial<AnsweredLeave>): AnsweredLeave {
	return {
		leaveID: 'leave-one',
		personID: 'member-one',
		person: '이샘플',
		kindID: 'leave',
		kind: '연차',
		days: 1,
		status: 'approved',
		isPaid: true,
		isDeducted: true,
		startDate: '2026-08-04',
		endDate: '2026-08-04',
		startsAt: '2026-08-03T15:00:00.000Z',
		endsAt: '2026-08-04T15:00:00.000Z',
		note: null,
		...leave
	};
}

function answeredAttendance(event: Partial<AnsweredAttendance>): AnsweredAttendance {
	return {
		eventID: 'attendance-one',
		personID: 'member-one',
		person: '이샘플',
		kind: 'clock_in',
		date: '2026-09-01',
		time: '08:30',
		occurredAt: '2026-08-31T23:30:00.000Z',
		location: '재택',
		wasCorrected: false,
		originalDate: null,
		originalTime: null,
		originalOccurredAt: null,
		reason: null,
		...event
	};
}

mock.module('../../../src/lib/public-api-call', () => ({
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		asked.push({ name, input });
		if (name === 'attendance_leave_policy_get') return defaultLeavePolicy();
		if (name === 'company_settings_get') {
			return {
				name: '샘플 주식회사',
				locale: 'ko',
				timeZone: 'Asia/Seoul',
				currencyCode: 'KRW',
				workLocations: [],
				leaveDays: 15,
				teamViewVisibleToAll: true,
				profileImageURL: null
			};
		}
		if (name === 'person_list') {
			return {
				requesterID: 'member-one',
				count: 1,
				people: [
					{
						personID: 'member-one',
						name: '이샘플',
						email: 'sample@example.com',
						isAdmin: false
					}
				]
			};
		}
		if (name === 'attendance_list') {
			return {
				scope: 'everyone',
				personID: null,
				personName: '',
				from: String(input.from),
				to: String(input.to),
				serverTime: databaseServerTime,
				backdatedAfterMinutes: 60,
				count: attendanceOfTheCompany.length,
				attendance: attendanceOfTheCompany
			};
		}
		if (name === 'leave_list') {
			const leave = input.scope === 'all' ? leaveOfTheCompany : leaveCoveringTheMoment;
			return { count: leave.length, leave, registeredKinds: ['연차'] };
		}
		throw new Error(`Unexpected tool ${name}`);
	}
}));

const { supabaseAttendanceSummary } = await import('../../../src/lib/attendance/supabase-attendance');
const { computeDayEvents, statusForDay } = await import(
	'../../../src/routes/attendance/shared/attendance-aggregation'
);

function attendanceListInput(): Record<string, unknown> {
	const call = asked.find((one) => one.name === 'attendance_list');
	if (!call) throw new Error('the summary never asked the record for attendance');
	return call.input;
}

describe('supabaseAttendanceSummary', () => {
	beforeEach(() => {
		leaveOfTheCompany = [];
		leaveCoveringTheMoment = [];
		attendanceOfTheCompany = [];
		asked.length = 0;
	});

	test('asks the record for the month and the day before it, for everybody', async () => {
		await supabaseAttendanceSummary('2026-09');

		expect(attendanceListInput()).toEqual({ scope: 'all', from: '2026-08-31', to: '2026-09-30' });
	});

	test('asks for the approved leave the month covers', async () => {
		await supabaseAttendanceSummary('2026-09');

		const call = asked.find((one) => one.name === 'leave_list' && one.input.scope === 'all');
		expect(call?.input).toEqual({
			scope: 'all',
			status: 'approved',
			from: '2026-09-01',
			to: '2026-09-30'
		});
	});

	// The record answers the day and the time in the company time zone. The
	// screen carries them; recomputing them here is how the two drift.
	test('carries the day and time the record answered rather than deriving them', async () => {
		attendanceOfTheCompany = [
			answeredAttendance({ date: '2026-09-01', time: '08:30', occurredAt: '2026-08-31T23:30:00.000Z' })
		];

		const summary = await supabaseAttendanceSummary('2026-09');

		expect(summary.events.map((event) => [event.localDate, event.localTime])).toEqual([
			['2026-09-01', '08:30']
		]);
		expect(summary.events[0].occurredAt).toBe('2026-08-31T23:30:00.000Z');
	});

	test('reaches back a day so an overnight clock in is still open this morning', async () => {
		attendanceOfTheCompany = [
			answeredAttendance({
				eventID: 'attendance-overnight',
				date: '2026-08-31',
				time: '22:00',
				occurredAt: '2026-08-31T13:00:00.000Z'
			})
		];

		const summary = await supabaseAttendanceSummary('2026-09');

		const morning = new Date('2026-09-01T09:00:00+09:00');
		const today = computeDayEvents('2026-09-01', summary.events, {
			currentDate: '2026-09-01',
			now: morning
		});
		expect(today.inProgress).toBe(true);
		expect(statusForDay('2026-09-01', summary.events, [], '2026-09-01', morning)).toBe('working');
	});

	test('reports no active leave when nothing covers the moment', async () => {
		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.activeLeave).toBeUndefined();
	});

	test('reports the leave covering the moment in the company time zone', async () => {
		leaveCoveringTheMoment = [
			answeredLeave({
				days: 0.5,
				startDate: '2026-08-18',
				endDate: '2026-08-18',
				startsAt: '2026-08-18T00:30:00.000Z',
				endsAt: '2026-08-18T04:30:00.000Z'
			})
		];

		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.activeLeave?.requestID).toBe('leave-one');
		expect(summary.activeLeave?.occurrenceID).toBe('leave-one');
		expect(summary.activeLeave?.startTime).toBe('09:30');
		expect(summary.activeLeave?.endTime).toBe('13:30');
		expect(summary.activeLeave?.deductionMilliDays).toBe(500);
	});

	test("uses the record's clock instead of the browser's", async () => {
		const browserTime = new Date('2030-01-01T00:00:00.000Z');
		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.serverTime).toBe(databaseServerTime);
		expect(summary.serverTime).not.toBe(browserTime.toISOString());
		expect(summary.backdatedAfterMinutes).toBe(60);
	});

	test('spreads a leave over every day the record says it covers', async () => {
		leaveOfTheCompany = [
			answeredLeave({ days: 3, startDate: '2026-08-04', endDate: '2026-08-06' })
		];

		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.absences.map((absence) => absence.date)).toEqual([
			'2026-08-04',
			'2026-08-05',
			'2026-08-06'
		]);
		expect(summary.absences.at(-1)?.isRangeEnd).toBe(true);
	});

	test('keeps a leave that ends the same day on that one day', async () => {
		leaveOfTheCompany = [
			answeredLeave({ days: 0.5, startDate: '2026-08-04', endDate: '2026-08-04' })
		];

		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.absences.map((absence) => absence.date)).toEqual(['2026-08-04']);
	});

	test('takes the workplaces and the team view rule from the company settings', async () => {
		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.timeZone).toBe('Asia/Seoul');
		expect(summary.teamViewVisibleToAll).toBe(true);
		expect(summary.currentUserEmail).toBe('sample@example.com');
		expect(summary.currentMemberID).toBe('member-one');
	});
});
