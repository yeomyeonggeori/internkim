import { beforeEach, describe, expect, mock, test } from 'bun:test';

const databaseServerTime = '2026-08-18T03:04:05.678Z';

type AttendanceRow = {
	id: string;
	member_id: string;
	kind: 'clock_in' | 'clock_out';
	location: string | null;
	occurred_at: string;
	original_occurred_at: string | null;
};

type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_paid: boolean;
	starts_at: string;
	ends_at: string;
	note: string | null;
};

let leaveRows: LeaveRow[] = [];
let coveringLeaveRows: LeaveRow[] = [];
let attendanceRows: AttendanceRow[] = [];
let attendanceWindow: { from: string; until: string } | null = null;

function leaveRow(startsAt: string, endsAt: string): LeaveRow {
	return {
		id: 'leave-one',
		member_id: 'member-one',
		kind: 'leave',
		is_paid: true,
		starts_at: startsAt,
		ends_at: endsAt,
		note: null
	};
}

function response<Value>(data: Value): Promise<{ data: Value; error: null }> {
	return Promise.resolve({ data, error: null });
}

const client = {
	auth: {
		getSession: () => response({ session: { user: { id: 'account-one' } } })
	},
	from(table: string) {
		if (table === 'company') {
			return {
				select: () => ({ limit: () => ({ single: () => response({ id: 'company-one', timezone: 'Asia/Seoul', work_locations: [], rules: {} }) }) })
			};
		}
		if (table === 'member') {
			return {
				select: () => ({
					neq: () => ({
						returns: () => response([{ id: 'member-one', name: '이샘플', email: 'sample@example.com', is_admin: false, user_id: 'account-one', joined_at: null }])
					})
				})
			};
		}
		if (table === 'attendance') {
			return {
				select: () => ({
					gte: (_column: string, from: string) => ({
						lt: (_lessThanColumn: string, until: string) => {
							attendanceWindow = { from, until };
							const inWindow = attendanceRows.filter(
								(row) => row.occurred_at >= from && row.occurred_at < until
							);
							return { order: () => ({ returns: () => response(inWindow) }) };
						}
					})
				})
			};
		}
		return {
			select: () => ({
				eq: () => ({
					lt: () => ({ gte: () => ({ returns: () => response(leaveRows) }) }),
					eq: () => ({
						lte: () => ({
							gt: () => ({
								order: () => ({ limit: () => ({ returns: () => response(coveringLeaveRows) }) })
							})
						})
					})
				})
			})
		};
	},
	rpc: (name: string) => {
		if (name === 'attendance_server_time') return response(databaseServerTime);
		if (name === 'attendance_correction_window_minutes') return response(60);
		throw new Error(`Unexpected RPC ${name}`);
	}
};

mock.module('$lib/supabase', () => ({ supabase: () => client }));

const { supabaseAttendanceSummary } = await import('../../../src/lib/attendance/supabase-attendance');

describe('supabaseAttendanceSummary', () => {
	beforeEach(() => {
		leaveRows = [];
		coveringLeaveRows = [];
		attendanceRows = [];
		attendanceWindow = null;
	});

	test('asks for the month as the company time zone bounds it', async () => {
		await supabaseAttendanceSummary('2026-09');

		expect(attendanceWindow).toEqual({
			from: '2026-08-31T15:00:00.000Z',
			until: '2026-09-30T15:00:00.000Z'
		});
	});

	test('carries a clock in made before the UTC day began on the first of the month', async () => {
		attendanceRows = [
			{
				id: 'attendance-one',
				member_id: 'member-one',
				kind: 'clock_in',
				location: '재택',
				occurred_at: '2026-08-31T23:30:00.000Z',
				original_occurred_at: null
			}
		];

		const summary = await supabaseAttendanceSummary('2026-09');

		expect(summary.events.map((event) => [event.localDate, event.localTime])).toEqual([
			['2026-09-01', '08:30']
		]);
	});

	test('reports no active leave when nothing covers the moment', async () => {
		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.activeLeave).toBeUndefined();
	});

	test('reports the leave covering the moment in the company time zone', async () => {
		coveringLeaveRows = [
			{
				...leaveRow('2026-08-18T00:30:00.000Z', '2026-08-18T04:30:00.000Z'),
				days: 0.5
			} as LeaveRow & { days: number }
		];

		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.activeLeave?.requestID).toBe('leave-one');
		expect(summary.activeLeave?.occurrenceID).toBe('leave-one');
		expect(summary.activeLeave?.startTime).toBe('09:30');
		expect(summary.activeLeave?.endTime).toBe('13:30');
		expect(summary.activeLeave?.deductionMilliDays).toBe(500);
	});

	test('uses the database RPC timestamp instead of the browser clock', async () => {
		const browserTime = new Date('2030-01-01T00:00:00.000Z');
		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.serverTime).toBe(databaseServerTime);
		expect(summary.serverTime).not.toBe(browserTime.toISOString());
		expect(summary.correctionWindowMinutes).toBe(60);
	});

	test('stops a full-day leave on the last day it covers', async () => {
		leaveRows = [leaveRow('2026-08-03T15:00:00.000Z', '2026-08-06T15:00:00.000Z')];

		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.absences.map((absence) => absence.date)).toEqual([
			'2026-08-04',
			'2026-08-05',
			'2026-08-06'
		]);
		expect(summary.absences.at(-1)?.isRangeEnd).toBe(true);
	});

	test('keeps a leave that ends the same day on that one day', async () => {
		leaveRows = [leaveRow('2026-08-04T00:00:00.000Z', '2026-08-04T09:00:00.000Z')];

		const summary = await supabaseAttendanceSummary('2026-08');

		expect(summary.absences.map((absence) => absence.date)).toEqual(['2026-08-04']);
	});
});
