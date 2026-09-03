import { describe, expect, test } from 'bun:test';
import { calendarEventFromApprovedLeave } from '../../src/lib/calendar/supabase-calendar-leave';
import { dayOffColor } from '../../src/lib/calendar/day-off-color';

describe('a day off is one colour', () => {
	test('an approved leave becomes a calendar event in that colour', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'leave-1',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
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
