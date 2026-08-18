import { describe, expect, test } from 'bun:test';
import { calendarEventFromApprovedLeave } from '../../src/lib/calendar/supabase-calendar-leave';
import { dayOffColor } from '../../src/lib/calendar/day-off-color';

const holidayStore = 'internal/admind/calendar_holiday_store.go';

describe('a day off is one colour', () => {
	test('leave wears the colour holidays are stored with', async () => {
		const source = await Bun.file(new URL(`../../../${holidayStore}`, import.meta.url)).text();
		const declared = source.match(/calendarHolidayColor\s*=\s*"(#[0-9a-fA-F]{6})"/);
		expect(declared?.[1]).toBe(dayOffColor);
	});

	test('an approved leave becomes a calendar event in that colour', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'leave-1',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-06-19T00:00:00.000Z',
				ends_at: '2026-06-19T23:59:59.000Z'
			},
			new Map([['member-1', { id: 'member-1', name: '이샘플', email: 'sample@example.com' }]]),
			'Asia/Seoul'
		);
		expect(event.color).toBe(dayOffColor);
		expect(event.readOnly).toBe(true);
	});
});
