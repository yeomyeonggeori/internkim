import { describe, expect, test } from 'bun:test';
import type { AttendanceEvent, AttendanceSummary } from '../../../src/routes/attendance/attendance-context.svelte';
import {
	attendanceAdditionWriteOutcome,
	attendanceEventsWriteOutcome,
	editableAttendanceEventIDs
} from '../../../src/routes/attendance/team/attendance-correction-access';
import type { TeamStatusPersonDaySegment } from '../../../src/routes/attendance/team/team-status-table-model';

const currentTime = new Date('2026-07-15T10:30:00+09:00');
const segment: TeamStatusPersonDaySegment = {
	id: 'segment-1',
	startEventID: 'clock-in',
	endEventID: 'clock-out',
	endReason: 'clock_out',
	locationName: '사무실',
	locationID: 'office',
	locationColor: '#22c55e',
	startTime: '09:00',
	endTime: '10:00',
	durationMinutes: 60,
	widthPercent: 12.5,
	isOpen: false
};

describe('attendance correction access', () => {
	test('keeps an own record reachable outside the three days, as one the administrators hear about', () => {
		const summary = summaryWith(
			{
				id: 'clock-in',
				occurredAt: '2026-07-15T10:25:00+09:00',
				originalOccurredAt: '2026-07-15T09:00:00+09:00'
			},
			{ id: 'clock-out', occurredAt: '2026-07-15T10:00:00+09:00' }
		);
		expect(editableAttendanceEventIDs(summary, [segment], currentTime)).toEqual(
			new Set(['clock-in', 'clock-out'])
		);
		expect(attendanceEventsWriteOutcome(summary, ['clock-in'], currentTime)).toBe('asked');
		expect(attendanceEventsWriteOutcome(summary, ['clock-out'], currentTime)).toBe('saved');
		expect(attendanceEventsWriteOutcome(summary, ['clock-in', 'clock-out'], currentTime)).toBe(
			'asked'
		);
	});

	test('leaves a colleague record out of reach for a member', () => {
		const summary = summaryWith(
			{ id: 'clock-in', email: 'colleague@example.com' },
			{ id: 'clock-out', email: 'colleague@example.com' }
		);
		expect(editableAttendanceEventIDs(summary, [segment], currentTime)).toEqual(new Set<string>());
		expect(attendanceEventsWriteOutcome(summary, ['clock-in'], currentTime)).toBe('blocked');
	});

	test('reads a record being added through the same threshold', () => {
		const summary = summaryWith({ id: 'clock-in' });
		expect(
			attendanceAdditionWriteOutcome(
				summary,
				{ email: 'member@example.com', localDate: '2026-07-15', localTime: '10:00' },
				currentTime
			)
		).toBe('saved');
		expect(
			attendanceAdditionWriteOutcome(
				summary,
				{ email: 'member@example.com', localDate: '2026-07-01', localTime: '09:00' },
				currentTime
			)
		).toBe('asked');
		expect(
			attendanceAdditionWriteOutcome(
				summary,
				{ email: 'colleague@example.com', localDate: '2026-07-15', localTime: '09:00' },
				currentTime
			)
		).toBe('blocked');
	});

	test('allows an administrator to correct another member regardless of event age', () => {
		const summary = summaryWith({
			id: 'clock-in',
			email: 'colleague@example.com',
			occurredAt: '2026-07-01T09:00:00+09:00'
		});
		summary.isAdmin = true;
		expect(editableAttendanceEventIDs(summary, [segment], currentTime)).toEqual(new Set(['clock-in']));
	});

});

function summaryWith(...overrides: Partial<AttendanceEvent>[]): AttendanceSummary {
	return {
		month: '2026-07',
		currentUserEmail: 'member@example.com',
		isAdmin: false,
		timeZone: 'Asia/Seoul',
		events: overrides.map(eventOf),
		absences: [],
		members: [],
		todayStatus: 'done',
		locations: [],
		teamViewVisibleToAll: true,
		teamViewBlocked: false,
		backdatedAfterMinutes: 60
	};
}

function eventOf(overrides: Partial<AttendanceEvent>): AttendanceEvent {
	return {
		id: 'clock-in',
		email: 'member@example.com',
		displayName: '구성원',
		kind: 'clock_in',
		occurredAt: '2026-07-15T10:00:00+09:00',
		localDate: '2026-07-15',
		localTime: '10:00',
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'web',
		resultPostID: '',
		...overrides
	};
}
