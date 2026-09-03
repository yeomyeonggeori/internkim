import { describe, expect, test } from 'bun:test';
import { calendarEntryOfApprovedLeave } from '../../../../../src/lib/server/public-api/record/company-calendar';

const members = new Map([
	['member-1', { id: 'member-1', name: '이샘플', email: 'sample@example.com' }]
]);

describe('calendarEntryOfApprovedLeave', () => {
	test('ends an all-day leave on the last day it covers, not the midnight after it', () => {
		const event = calendarEntryOfApprovedLeave(
			{
				id: 'leave-1',
				member_id: 'member-1',
				kind: '연차',
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
			startISO: '2026-08-02T15:00:00.000Z',
			endISO: '2026-08-03T14:59:59.999Z',
			isAllDay: true,
			readOnly: true,
			source: 'leave'
		});
	});

	test('ends a whole day on the day it covers, west of UTC as well', () => {
		const event = calendarEntryOfApprovedLeave(
			{
				id: 'full-day-west',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
				starts_at: '2026-08-03T07:00:00.000Z',
				ends_at: '2026-08-04T07:00:00.000Z'
			},
			members,
			'America/Los_Angeles'
		);

		expect(event).toMatchObject({
			title: '이샘플 · 휴가',
			startISO: '2026-08-03T07:00:00.000Z',
			endISO: '2026-08-04T06:59:59.999Z',
			isAllDay: true
		});
	});

	test('maps partial leave to a timed event without a note field', () => {
		const event = calendarEntryOfApprovedLeave(
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

	test('labels half-day and quarter-day leave by their deduction', () => {
		const halfDay = calendarEntryOfApprovedLeave(
			{
				id: 'leave-half-day',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.5,
				status: 'approved',
				starts_at: '2026-08-03T05:00:00.000Z',
				ends_at: '2026-08-03T09:00:00.000Z'
			},
			members,
			'Asia/Seoul'
		);
		const quarterDay = calendarEntryOfApprovedLeave(
			{
				id: 'leave-quarter-day',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.25,
				status: 'approved',
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
		const fullDay = calendarEntryOfApprovedLeave(
			{
				id: 'leave-full-day-en',
				member_id: 'member-1',
				kind: 'leave',
				days: 1,
				status: 'approved',
				starts_at: '2026-08-02T15:00:00.000Z',
				ends_at: '2026-08-03T15:00:00.000Z'
			},
			members,
			'Asia/Seoul',
			'en'
		);
		const halfDay = calendarEntryOfApprovedLeave(
			{
				id: 'leave-half-day-en',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.5,
				status: 'approved',
				starts_at: '2026-08-03T05:00:00.000Z',
				ends_at: '2026-08-03T09:00:00.000Z'
			},
			members,
			'Asia/Seoul',
			'en'
		);
		const quarterDay = calendarEntryOfApprovedLeave(
			{
				id: 'leave-quarter-day-en',
				member_id: 'member-1',
				kind: 'leave',
				days: 0.25,
				status: 'approved',
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
		const event = calendarEntryOfApprovedLeave(
			{
				id: 'unknown-member-en',
				member_id: 'unknown-member',
				kind: 'leave',
				days: 1,
				status: 'approved',
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
		const event = calendarEntryOfApprovedLeave(
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
