import { describe, expect, test } from 'bun:test';
import type { AttendanceEvent, AttendanceSummary } from '../../../src/routes/attendance/attendance-context.svelte';
import { editableAttendanceEventIDs } from '../../../src/routes/attendance/team/attendance-correction-access';
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
	test('allows a member only within the original event time window', () => {
		const summary = summaryWith(
			{
				id: 'clock-in',
				occurredAt: '2026-07-15T10:25:00+09:00',
				originalOccurredAt: '2026-07-15T09:00:00+09:00'
			},
			{ id: 'clock-out', occurredAt: '2026-07-15T10:00:00+09:00' }
		);
		expect(editableAttendanceEventIDs(summary, [segment], currentTime)).toEqual(new Set(['clock-out']));
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

	test('keeps device-mode events editable without applying the central policy', () => {
		const summary = summaryWith({ id: 'clock-in', occurredAt: '2026-07-01T09:00:00+09:00' });
		delete summary.correctionWindowMinutes;
		expect(editableAttendanceEventIDs(summary, [segment], currentTime)).toEqual(
			new Set(['clock-in', 'clock-out'])
		);
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
		correctionWindowMinutes: 60
	};
}

function eventOf(overrides: Partial<AttendanceEvent>): AttendanceEvent {
	return {
		id: 'clock-in',
		mattermostUserID: '',
		mattermostUsername: '',
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
