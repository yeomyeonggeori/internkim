import { expect, test } from 'bun:test';
import { currentAttendanceSummaryOf } from '../../../src/lib/attendance/supabase-current-attendance';
import { clockInNobodyClosed } from '../../../src/routes/attendance/shared/attendance-work-segments';
import { attendanceSummaryRecords } from '../../../src/lib/attendance/attendance-summary-records';
import type { CurrentAttendance } from '../../../src/lib/attendance/current-attendance';
import type { SavedAttendanceEvent } from '../../../src/lib/attendance/recorded-attendance';

const state: CurrentAttendance = {
	memberID: 'member', email: 'sample@example.com', companyID: 'company', timeZone: 'America/New_York',
	serverTime: '2026-11-01T06:30:00Z', backdatedAfterMinutes: 3,
	workLocations: [{ name: 'Office', color: null }],
	authorization: { isAdmin: false, teamViewVisibleToAll: false }, todayEvents: [],
	latestEvent: { id: 'old', personID: 'member', kind: 'clock_in', occurredAt: '2026-09-30T23:00:00Z', location: 'Office' },
	activeLeave: { leaveID: 'leave', kindID: 'annual', kindName: null, days: 0.5, startsAt: '2026-11-01T05:00:00Z', endsAt: '2026-11-01T07:00:00Z' }
};

test('own snapshot preserves an older open shift and never supplies team live records', () => {
	const summary = currentAttendanceSummaryOf(state);
	expect(summary.readScope).toBe('mine');
	expect(summary[attendanceSummaryRecords]).toBeUndefined();
	expect(clockInNobodyClosed(summary.events)?.id).toBe('old');
	expect(summary.members).toHaveLength(1);
	expect(summary.teamViewVisibleToAll).toBe(false);
});

test('active partial leave and repeated DST hour use company time and historical labels', () => {
	const summary = currentAttendanceSummaryOf(state);
	expect(summary.month).toBe('2026-11');
	expect(summary.activeLeave).toMatchObject({ requestID: 'leave', deductionMilliDays: 500, startTime: '01:00', endTime: '02:00', leaveTypeName: '연차' });
	expect(summary.absences[0]?.date).toBe('2026-11-01');
});

test('today latest event is represented once and a close removes the previous-open indication', () => {
	const latestEvent: SavedAttendanceEvent = { id: 'closed', personID: 'member', kind: 'clock_out', occurredAt: '2026-11-01T06:00:00Z', location: null };
	const summary = currentAttendanceSummaryOf({ ...state, latestEvent, todayEvents: [latestEvent], activeLeave: null });
	expect(summary.events).toHaveLength(1);
	expect(clockInNobodyClosed(summary.events)).toBeUndefined();
	expect(summary.activeLeave).toBeUndefined();
});
