import { describe, expect, test } from 'bun:test';
import { clockInNobodyClosed } from '../../../src/routes/attendance/shared/attendance-work-segments';
import type { AttendanceEvent } from '../../../src/routes/attendance/attendance-context.svelte';

function event(kind: 'clock_in' | 'clock_out', occurredAt: string, canceledAt?: string): AttendanceEvent {
	return {
		id: `${kind}-${occurredAt}`,
		mattermostUserID: '',
		mattermostUsername: '',
		email: 'sample@example.com',
		displayName: '이샘플',
		kind,
		occurredAt,
		localDate: occurredAt.slice(0, 10),
		localTime: occurredAt.slice(11, 16),
		locationID: '',
		canceledAt,
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'web',
		resultPostID: ''
	};
}

// public.attendance_add reads the newest event and refuses a clock-in that
// repeats one (20260901000004_a_refusal_becomes_a_request.sql). The screen has
// to reach the same answer, or it offers a button the record will refuse.
describe('the clock-in nobody closed', () => {
	test('is the newest event when that event is a clock-in', () => {
		const stillOpen = clockInNobodyClosed([
			event('clock_in', '2026-09-01T00:00:00Z'),
			event('clock_out', '2026-09-01T09:00:00Z'),
			event('clock_in', '2026-09-02T00:00:00Z')
		]);
		expect(stillOpen?.occurredAt).toBe('2026-09-02T00:00:00Z');
	});

	test('is nobody when the newest event is a clock-out', () => {
		expect(
			clockInNobodyClosed([
				event('clock_in', '2026-09-02T00:00:00Z'),
				event('clock_out', '2026-09-02T09:00:00Z')
			])
		).toBeUndefined();
	});

	test('is nobody when nothing has been recorded', () => {
		expect(clockInNobodyClosed([])).toBeUndefined();
	});

	test('reads the newest by when it happened, not by the order it was given', () => {
		const stillOpen = clockInNobodyClosed([
			event('clock_in', '2026-09-02T00:00:00Z'),
			event('clock_out', '2026-09-01T09:00:00Z'),
			event('clock_in', '2026-09-01T00:00:00Z')
		]);
		expect(stillOpen?.occurredAt).toBe('2026-09-02T00:00:00Z');
	});

	test('does not count an event somebody took back', () => {
		expect(
			clockInNobodyClosed([
				event('clock_in', '2026-09-01T00:00:00Z'),
				event('clock_out', '2026-09-01T09:00:00Z'),
				event('clock_in', '2026-09-02T00:00:00Z', '2026-09-02T01:00:00Z')
			])
		).toBeUndefined();
	});
});
