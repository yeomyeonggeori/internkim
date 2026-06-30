import { expect, test } from 'bun:test';

import { calendarDateTimeRangeChangesForStart } from '../../../src/routes/calendar/embed/calendar-date-time-range';

test('keeps the existing duration when a timed range start moves after the current end', () => {
	const changes = calendarDateTimeRangeChangesForStart(
		{
			startDateKey: '2026-06-18',
			endDateKey: '2026-06-18',
			startTime: '10:00',
			endTime: '12:00',
			allDay: false
		},
		{
			startDateKey: '2026-06-18',
			startTime: '14:00'
		}
	);

	expect(changes).toEqual({
		startDateKey: '2026-06-18',
		endDateKey: '2026-06-18',
		startTime: '14:00',
		endTime: '16:00',
		allDay: false
	});
});

test('keeps the existing duration when a timed range start moves across dates', () => {
	const changes = calendarDateTimeRangeChangesForStart(
		{
			startDateKey: '2026-06-18',
			endDateKey: '2026-06-19',
			startTime: '23:00',
			endTime: '01:00',
			allDay: false
		},
		{
			startDateKey: '2026-06-20',
			startTime: '10:00'
		}
	);

	expect(changes).toEqual({
		startDateKey: '2026-06-20',
		endDateKey: '2026-06-20',
		startTime: '10:00',
		endTime: '12:00',
		allDay: false
	});
});
