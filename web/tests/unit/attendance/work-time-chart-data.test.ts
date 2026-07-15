import { describe, expect, test } from 'bun:test';
import type { AttendanceEvent } from '../../../src/routes/attendance/attendance-context.svelte';
import { buildDailyWorkTimeValues } from '../../../src/routes/attendance/shared/work-time-chart-data';

describe('work time chart data', () => {
	test('counts an open overnight segment from midnight on its second day', () => {
		const clockIn: AttendanceEvent = {
			id: 'remote-in',
			mattermostUserID: 'staff-1',
			mattermostUsername: 'staff',
			email: 'staff@example.com',
			displayName: 'Staff',
			kind: 'clock_in',
			occurredAt: '2026-06-01T22:00:00+09:00',
			localDate: '2026-06-01',
			localTime: '22:00:00',
			timeZoneAtEvent: 'Asia/Seoul',
			source: 'test',
			resultPostID: 'remote-in-post',
			locationID: 'remote',
			locationName: 'Remote',
		};

		const values = buildDailyWorkTimeValues('2026-06', [clockIn], {
			currentDate: '2026-06-02',
			fallbackLocationName: 'Unknown',
			now: new Date('2026-06-02T01:00:00+09:00'),
		});

		expect(values.find((value) => value.date === '2026-06-01')?.minutesByLocation).toEqual({ Remote: 120 });
		expect(values.find((value) => value.date === '2026-06-02')?.minutesByLocation).toEqual({ Remote: 60 });
	});
});
