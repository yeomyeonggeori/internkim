import { describe, expect, test } from 'bun:test';
import type { AttendanceEvent } from '../../../src/routes/attendance/attendance-context.svelte';
import { computeDayEvents } from '../../../src/routes/attendance/shared/attendance-day-events';

describe('attendance work segments', () => {
	test('pairs multiple location visits without counting time between segments', () => {
		const events = [
			attendanceEvent('home-in', 'clock_in', '2026-06-17T01:30:00.000Z', '10:30:00', 'home', '재택'),
			attendanceEvent('home-out', 'clock_out', '2026-06-17T03:00:00.000Z', '12:00:00'),
			attendanceEvent('office-in', 'clock_in', '2026-06-17T08:00:00.000Z', '17:00:00', 'office', '사무실'),
			attendanceEvent('office-out', 'clock_out', '2026-06-17T09:00:00.000Z', '18:00:00'),
		];

		const day = computeDayEvents('2026-06-17', events);

		expect(day.segments.length).toBe(2);
		expect(day.workedMinutes).toBe(150);
		expect(day.segments.map((segment) => segment.locationName)).toEqual(['재택', '사무실']);
		expect(day.segments.map((segment) => segment.workedMinutes)).toEqual([90, 60]);
		expect(day.clockIn?.localTime).toBe('17:00:00');
		expect(day.clockOut?.localTime).toBe('18:00:00');
	});

	test('marks the latest unclosed location as the active segment', () => {
		const events = [
			attendanceEvent('home-in', 'clock_in', '2026-06-17T01:30:00.000Z', '10:30:00', 'home', '재택'),
			attendanceEvent('home-out', 'clock_out', '2026-06-17T03:00:00.000Z', '12:00:00'),
			attendanceEvent('field-in', 'clock_in', '2026-06-17T09:00:00.000Z', '18:00:00', 'field', '외부'),
		];

		const day = computeDayEvents('2026-06-17', events);

		expect(day.inProgress).toBe(true);
		expect(day.activeSegment?.locationName).toBe('외부');
		expect(day.activeSegment?.startTime).toBe('18:00:00');
		expect(day.clockIn?.localTime).toBe('18:00:00');
		expect(day.workedMinutes).toBe(90);
	});

	test('uses the next clock-in as the previous location end when location changes without clock-out', () => {
		const events = [
			attendanceEvent('office-in', 'clock_in', '2026-06-17T00:00:00.000Z', '09:00:00', 'office', '사무실'),
			attendanceEvent('home-in', 'clock_in', '2026-06-17T02:00:00.000Z', '11:00:00', 'home', '재택'),
		];

		const day = computeDayEvents('2026-06-17', events);

		expect(day.segments.length).toBe(2);
		expect(day.segments[0]).toMatchObject({
			locationName: '사무실',
			endTime: '11:00:00',
			workedMinutes: 120,
			isOpen: false,
			endReason: 'next_clock_in',
		});
		expect(day.activeSegment?.locationName).toBe('재택');
		expect(day.workedMinutes).toBe(120);
	});

	test('splits overnight work at midnight for each display date', () => {
		const events = [
			attendanceEvent('night-in', 'clock_in', '2026-06-01T22:00:00+09:00', '22:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('night-out', 'clock_out', '2026-06-02T02:00:00+09:00', '02:00:00', 'office', '사무실', {
				localDate: '2026-06-02',
			}),
		];

		const firstDay = computeDayEvents('2026-06-01', events);
		const secondDay = computeDayEvents('2026-06-02', events);

		expect(firstDay.inProgress).toBe(false);
		expect(firstDay.workedMinutes).toBe(120);
		expect(firstDay.segments.length).toBe(1);
		expect(firstDay.segments[0]).toMatchObject({
			startTime: '22:00:00',
			endTime: '24:00:00',
			workedMinutes: 120,
			isOpen: false,
		});
		expect(secondDay.inProgress).toBe(false);
		expect(secondDay.workedMinutes).toBe(120);
		expect(secondDay.segments.length).toBe(1);
		expect(secondDay.segments[0]).toMatchObject({
			startTime: '00:00:00',
			endTime: '02:00:00',
			workedMinutes: 120,
			isOpen: false,
		});
	});

	test('keeps an open overnight segment active on the current display date', () => {
		const events = [
			attendanceEvent('night-in', 'clock_in', '2026-06-01T22:00:00+09:00', '22:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
		];

		const firstDay = computeDayEvents('2026-06-01', events, { currentDate: '2026-06-02' });
		const secondDay = computeDayEvents('2026-06-02', events, { currentDate: '2026-06-02' });

		expect(firstDay.inProgress).toBe(false);
		expect(firstDay.workedMinutes).toBe(120);
		expect(firstDay.segments[0]).toMatchObject({
			startTime: '22:00:00',
			endTime: '24:00:00',
			workedMinutes: 120,
			isOpen: false,
		});
		expect(secondDay.inProgress).toBe(true);
		expect(secondDay.activeSegment).toMatchObject({
			startTime: '00:00:00',
			endTime: undefined,
			workedMinutes: 0,
			isOpen: true,
		});
		expect(secondDay.clockIn?.localTime).toBe('22:00:00');
		expect(secondDay.clockOut).toBe(undefined);
	});

	test('ignores canceled events when building segments', () => {
		const events = [
			attendanceEvent('home-in', 'clock_in', '2026-06-17T01:30:00.000Z', '10:30:00', 'home', '재택', {
				canceledAt: '2026-06-17T01:31:00.000Z',
			}),
			attendanceEvent('home-out', 'clock_out', '2026-06-17T03:00:00.000Z', '12:00:00'),
			attendanceEvent('office-in', 'clock_in', '2026-06-17T04:00:00.000Z', '13:00:00', 'office', '사무실'),
		];

		const day = computeDayEvents('2026-06-17', events);

		expect(day.segments.length).toBe(1);
		expect(day.activeSegment?.locationName).toBe('사무실');
	});
});

function attendanceEvent(
	id: string,
	kind: AttendanceEvent['kind'],
	occurredAt: string,
	localTime: string,
	locationID = '',
	locationName = '',
	overrides: Partial<AttendanceEvent> = {}
): AttendanceEvent {
	return {
		id,
		mattermostUserID: 'user-1',
		mattermostUsername: 'user',
		email: 'user@example.com',
		displayName: 'User',
		kind,
		occurredAt,
		localDate: '2026-06-17',
		localTime,
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'test',
		resultPostID: `${id}-post`,
		locationID,
		locationName,
		...overrides,
	};
}
