import { describe, expect, test } from 'bun:test';
import { isScheduledWorkingDate } from '../../../src/lib/attendance/supabase-work-calendar';

const weekdaySchedule = [[
	[{ from: '09:00', to: '18:00' }],
	[{ from: '09:00', to: '18:00' }],
	[{ from: '09:00', to: '18:00' }],
	[{ from: '09:00', to: '18:00' }],
	[{ from: '09:00', to: '18:00' }],
	null,
	null
]];

describe('isScheduledWorkingDate', () => {
	test('uses the repeating work-hours cycle', () => {
		expect(isScheduledWorkingDate('2026-08-03', weekdaySchedule)).toBe(true);
		expect(isScheduledWorkingDate('2026-08-08', weekdaySchedule)).toBe(false);
	});

	test('treats an unconstrained schedule as available on every calendar date', () => {
		expect(isScheduledWorkingDate('2026-08-08', null)).toBe(true);
	});
});
