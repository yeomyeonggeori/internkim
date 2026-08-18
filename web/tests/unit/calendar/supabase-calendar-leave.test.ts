import { describe, expect, test } from 'bun:test';
import {
	calendarEventFromApprovedLeave,
	calendarEventsFromApprovedLeaveRows,
	type ApprovedLeaveRow
} from '../../../src/lib/calendar/supabase-calendar-leave';

const members = new Map([
	['member-1', { id: 'member-1', name: '이샘플', email: 'sample@example.com' }]
]);

describe('calendarEventFromApprovedLeave', () => {
	test('maps company-local midnight bounds to an all-day read-only event', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'leave-1',
				member_id: 'member-1',
				kind: '연차',
				days: 1,
				status: 'approved',
				cancelled_at: null,
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

	test('maps legacy UTC-midnight full-day bounds in a negative UTC offset', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'legacy-full-day',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-03T00:00:00.000Z',
				ends_at: '2026-08-04T00:00:00.000Z'
			},
			members,
			'America/Los_Angeles'
		);

		expect(event).toMatchObject({
			title: '이샘플 · 휴가',
			startISO: '2026-08-03T00:00:00.000Z',
			endISO: '2026-08-04T00:00:00.000Z',
			isAllDay: true
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
				cancelled_at: null,
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

	test('labels half-day and quarter-day leave by their deduction', () => {
		const halfDay = calendarEventFromApprovedLeave(
			{
				id: 'leave-half-day',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.5,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-03T05:00:00.000Z',
				ends_at: '2026-08-03T09:00:00.000Z'
			},
			members,
			'Asia/Seoul'
		);
		const quarterDay = calendarEventFromApprovedLeave(
			{
				id: 'leave-quarter-day',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.25,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-04T00:00:00.000Z',
				ends_at: '2026-08-04T02:00:00.000Z'
			},
			members,
			'Asia/Seoul'
		);

		expect(halfDay.title).toBe('이샘플 · 반차');
		expect(quarterDay.title).toBe('이샘플 · 반반차');
	});

	test('localizes full-day and partial leave labels in English', () => {
		const fullDay = calendarEventFromApprovedLeave(
			{
				id: 'leave-full-day-en',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-02T15:00:00.000Z',
				ends_at: '2026-08-03T15:00:00.000Z'
			},
			members,
			'Asia/Seoul',
			'en'
		);
		const halfDay = calendarEventFromApprovedLeave(
			{
				id: 'leave-half-day-en',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.5,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-03T05:00:00.000Z',
				ends_at: '2026-08-03T09:00:00.000Z'
			},
			members,
			'Asia/Seoul',
			'en'
		);
		const quarterDay = calendarEventFromApprovedLeave(
			{
				id: 'leave-quarter-day-en',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.25,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-04T00:00:00.000Z',
				ends_at: '2026-08-04T02:00:00.000Z'
			},
			members,
			'Asia/Seoul',
			'en'
		);

		expect(fullDay.title).toBe('이샘플 · Leave');
		expect(halfDay.title).toBe('이샘플 · Half-day leave');
		expect(quarterDay.title).toBe('이샘플 · Quarter-day leave');
	});

	test('localizes the fallback member name in English', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'unknown-member-en',
				member_id: 'unknown-member',
				kind: 'leave',
				days: 1,
				status: 'approved',
				cancelled_at: null,
				starts_at: '2026-08-02T15:00:00.000Z',
				ends_at: '2026-08-03T15:00:00.000Z'
			},
			new Map(),
			'Asia/Seoul',
			'en'
		);

		expect(event.title).toBe('Member · Leave');
	});

	test('keeps leave IDs separate from task IDs', () => {
		const event = calendarEventFromApprovedLeave(
			{
				id: 'same-id',
				member_id: 'member-1',
				kind: 'other',
				days: 0.5,
				status: 'approved',
				cancelled_at: null,
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

describe('calendarEventsFromApprovedLeaveRows', () => {
	test('excludes cancelled approved leave from calendar events', () => {
		const activeLeave: ApprovedLeaveRow = {
			id: 'active-leave',
			member_id: 'member-1',
			kind: 'leave',
			days: 1,
			status: 'approved',
			cancelled_at: null,
			starts_at: '2026-08-02T15:00:00.000Z',
			ends_at: '2026-08-03T15:00:00.000Z'
		};
		const events = calendarEventsFromApprovedLeaveRows(
			[
				activeLeave,
				{ ...activeLeave, id: 'cancelled-leave', cancelled_at: '2026-08-01T00:00:00.000Z' }
			],
			members,
			'Asia/Seoul'
		);

		expect(events.map((event) => event.id)).toEqual(['leave:active-leave']);
	});
});
