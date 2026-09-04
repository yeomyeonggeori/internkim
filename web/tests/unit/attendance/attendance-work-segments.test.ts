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

		const day = computeDayEvents('2026-06-17', events, { now: new Date('2026-06-17T10:00:00.000Z') });

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

		const day = computeDayEvents('2026-06-17', events, { now: new Date('2026-06-17T03:00:00.000Z') });

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

	test('restores an overnight remote segment whose legacy clock-out kept the clock-in date', () => {
		const events = [
			attendanceEvent('office-in', 'clock_in', '2026-06-01T09:00:00+09:00', '09:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('office-out', 'clock_out', '2026-06-01T18:00:00+09:00', '18:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('remote-in', 'clock_in', '2026-06-01T21:00:00+09:00', '21:00:00', 'remote', '재택', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('remote-out', 'clock_out', '2026-06-02T02:00:00+09:00', '02:00:00', 'remote', '재택', {
				localDate: '2026-06-01',
			}),
		];

		const firstDay = computeDayEvents('2026-06-01', events);
		const secondDay = computeDayEvents('2026-06-02', events);

		expect(firstDay.segments.map((segment) => segment.locationID)).toEqual(['office', 'remote']);
		expect(firstDay.segments[1]).toMatchObject({
			startTime: '21:00:00',
			endTime: '24:00:00',
			workedMinutes: 180,
		});
		expect(secondDay.segments.length).toBe(1);
		expect(secondDay.segments[0]).toMatchObject({
			locationID: 'remote',
			startTime: '00:00:00',
			endTime: '02:00:00',
			workedMinutes: 120,
		});
	});

	test('keeps an open overnight segment active on the current display date', () => {
		const events = [
			attendanceEvent('night-in', 'clock_in', '2026-06-01T22:00:00+09:00', '22:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
		];

		const options = { currentDate: '2026-06-02', now: new Date('2026-06-02T02:00:00+09:00') };
		const firstDay = computeDayEvents('2026-06-01', events, options);
		const secondDay = computeDayEvents('2026-06-02', events, options);

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

	test('shows a shift that has been open since last night', () => {
		const events = [
			attendanceEvent('night-in', 'clock_in', '2026-06-01T22:00:00+09:00', '22:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
		];

		const day = computeDayEvents('2026-06-02', events, {
			currentDate: '2026-06-02',
			now: new Date('2026-06-02T08:00:00+09:00'),
		});

		expect(day.inProgress).toBe(true);
		expect(day.activeSegment?.startTime).toBe('00:00:00');
	});

	test('shows nothing for a shift nobody closed for over a day', () => {
		const events = [
			attendanceEvent('forgotten-in', 'clock_in', '2026-06-01T09:00:00+09:00', '09:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
		];

		const options = { currentDate: '2026-06-03', now: new Date('2026-06-03T09:00:00+09:00') };

		expect(computeDayEvents('2026-06-03', events, options).inProgress).toBe(false);
		expect(computeDayEvents('2026-06-03', events, options).segments).toEqual([]);
		expect(computeDayEvents('2026-06-01', events, options).segments).toEqual([]);
	});

	test('shows nothing for a shift that was closed days after it began', () => {
		const events = [
			attendanceEvent('trip-in', 'clock_in', '2026-06-01T09:00:00+09:00', '09:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('trip-out', 'clock_out', '2026-06-04T18:00:00+09:00', '18:00:00', 'office', '사무실', {
				localDate: '2026-06-04',
			}),
		];

		const options = { currentDate: '2026-06-10', now: new Date('2026-06-10T09:00:00+09:00') };

		for (const date of ['2026-06-01', '2026-06-02', '2026-06-03', '2026-06-04']) {
			expect(computeDayEvents(date, events, options).segments).toEqual([]);
		}
	});

	test('shows a long night that was closed inside the day it may last', () => {
		const events = [
			attendanceEvent('night-in', 'clock_in', '2026-06-01T14:00:00+09:00', '14:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('night-out', 'clock_out', '2026-06-02T08:00:00+09:00', '08:00:00', 'office', '사무실', {
				localDate: '2026-06-02',
			}),
		];

		const options = { currentDate: '2026-06-10', now: new Date('2026-06-10T09:00:00+09:00') };

		expect(computeDayEvents('2026-06-01', events, options).workedMinutes).toBe(600);
		expect(computeDayEvents('2026-06-02', events, options).workedMinutes).toBe(480);
	});

	test('drops only the section that outlived its day, not the day around it', () => {
		const events = [
			attendanceEvent('home-in', 'clock_in', '2026-06-01T09:00:00+09:00', '09:00:00', 'home', '재택', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('home-out', 'clock_out', '2026-06-01T15:00:00+09:00', '15:00:00', 'home', '재택', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('office-in', 'clock_in', '2026-06-01T16:00:00+09:00', '16:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('office-out', 'clock_out', '2026-06-11T10:00:00+09:00', '10:00:00', 'office', '사무실', {
				localDate: '2026-06-11',
			}),
		];

		const options = { currentDate: '2026-06-12', now: new Date('2026-06-12T09:00:00+09:00') };
		const day = computeDayEvents('2026-06-01', events, options);

		expect(day.segments.map((segment) => segment.locationName)).toEqual(['재택']);
		expect(day.workedMinutes).toBe(360);
		expect(computeDayEvents('2026-06-05', events, options).segments).toEqual([]);
	});

	test('shows nothing for a shift the next clock in closed days later', () => {
		const events = [
			attendanceEvent('forgotten-in', 'clock_in', '2026-06-01T09:00:00+09:00', '09:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
			attendanceEvent('back-in', 'clock_in', '2026-06-05T09:00:00+09:00', '09:00:00', 'home', '재택', {
				localDate: '2026-06-05',
			}),
			attendanceEvent('back-out', 'clock_out', '2026-06-05T18:00:00+09:00', '18:00:00', 'home', '재택', {
				localDate: '2026-06-05',
			}),
		];

		const options = { currentDate: '2026-06-10', now: new Date('2026-06-10T09:00:00+09:00') };

		expect(computeDayEvents('2026-06-02', events, options).segments).toEqual([]);
		expect(computeDayEvents('2026-06-05', events, options).workedMinutes).toBe(540);
	});

	test('stops showing the shift the moment it outlives its day', () => {
		const events = [
			attendanceEvent('long-in', 'clock_in', '2026-06-01T09:00:00+09:00', '09:00:00', 'office', '사무실', {
				localDate: '2026-06-01',
			}),
		];

		const justUnder = computeDayEvents('2026-06-02', events, {
			currentDate: '2026-06-02',
			now: new Date('2026-06-02T08:59:00+09:00'),
		});
		const justOver = computeDayEvents('2026-06-02', events, {
			currentDate: '2026-06-02',
			now: new Date('2026-06-02T09:01:00+09:00'),
		});

		expect(justUnder.inProgress).toBe(true);
		expect(justOver.inProgress).toBe(false);
	});

	test('ignores canceled events when building segments', () => {
		const events = [
			attendanceEvent('home-in', 'clock_in', '2026-06-17T01:30:00.000Z', '10:30:00', 'home', '재택', {
				canceledAt: '2026-06-17T01:31:00.000Z',
			}),
			attendanceEvent('home-out', 'clock_out', '2026-06-17T03:00:00.000Z', '12:00:00'),
			attendanceEvent('office-in', 'clock_in', '2026-06-17T04:00:00.000Z', '13:00:00', 'office', '사무실'),
		];

		const day = computeDayEvents('2026-06-17', events, { now: new Date('2026-06-17T05:00:00.000Z') });

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
