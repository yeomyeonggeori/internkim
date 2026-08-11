import { describe, expect, test } from 'bun:test';
import { calendarEventFromApprovedLeave } from '../../../src/lib/calendar/supabase-calendar-leave';

const members = new Map([
	['member-1', { id: 'member-1', name: '이샘플', email: 'sample@example.com' }]
]);

describe('calendarEventFromApprovedLeave', () => {
	test('maps company-local midnight bounds to an all-day read-only event', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'leave-1',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
				starts_at: '2026-08-02T15:00:00.000Z',
				ends_at: '2026-08-03T15:00:00.000Z'
			},
			members,
			'Asia/Seoul'
		);

		expect(event).toMatchObject({
			id: 'leave:leave-1',
			uid: 'leave:leave-1',
			title: '이샘플 · 휴가',
			description: '',
			startISO: '2026-08-03T00:00:00.000Z',
			endISO: '2026-08-04T00:00:00.000Z',
			isAllDay: true,
			readOnly: true,
			source: 'leave'
		});
	});

	test('maps partial leave to a timed event without a note field', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'leave-2',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.25,
				status: 'approved',
				starts_at: '2026-08-03T02:30:00.000Z',
				ends_at: '2026-08-03T05:30:00.000Z'
			},
			members,
			'Asia/Seoul'
		);

		expect(event.isAllDay).toBe(false);
		expect(event.description).toBe('');
		expect(Object.keys(event)).not.toContain('note');
		expect(event.participants).toEqual([
			{ personID: 'member-1', name: '이샘플', email: 'sample@example.com' }
		]);
	});

	test('keeps leave IDs separate from task IDs', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'same-id',
				member_id: 'member-1',
				kind: 'other',
				days: 0.5,
				status: 'approved',
				starts_at: '2026-08-03T00:00:00.000Z',
				ends_at: '2026-08-03T05:00:00.000Z'
			},
			members,
			'Asia/Seoul'
		);

		expect(event.id).toBe('leave:same-id');
		expect(event.title).toBe('이샘플 · other');
	});
});
