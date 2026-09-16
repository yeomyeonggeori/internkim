import { expect, test } from 'bun:test';
import { attendanceEventFromClock } from '../../../src/lib/attendance/supabase-attendance';

test('turns the saved clock event into a screen event using the company time zone', () => {
	const event = attendanceEventFromClock(
		{
			id: 'event-one',
			personID: 'member-one',
			kind: 'clock_in',
			occurredAt: '2026-08-31T23:30:00.000Z',
			location: '재택'
		},
		'sample@example.com',
		'Asia/Seoul'
	);

	expect(event).toMatchObject({
		id: 'event-one',
		email: 'sample@example.com',
		kind: 'clock_in',
		localDate: '2026-09-01',
		localTime: '08:30',
		locationID: '재택',
		locationName: '재택'
	});
});
