import { describe, expect, test } from 'bun:test';
import {
	attendanceWrittenByHand,
	type AttendanceRow
} from '$lib/server/public-api/record/attendance';

function attendanceRow(row: Partial<AttendanceRow>): AttendanceRow {
	return {
		id: 'event-1',
		member_id: 'member-1',
		kind: 'clock_in',
		location: '사무실',
		occurred_at: '2026-09-01T00:00:00.000Z',
		original_occurred_at: null,
		edit_reason: null,
		...row
	};
}

describe('which attendance records were written by hand', () => {
	test('keeps a backdated write, which carries the reason it was written for', () => {
		const written = attendanceRow({ id: 'backdated', edit_reason: '출근 찍는 것을 잊었습니다' });
		expect(attendanceWrittenByHand([written])).toEqual([written]);
	});

	test('keeps a record that was moved, whether or not a reason came with it', () => {
		const moved = attendanceRow({
			id: 'moved',
			original_occurred_at: '2026-08-31T23:00:00.000Z'
		});
		expect(attendanceWrittenByHand([moved])).toEqual([moved]);
	});

	test('drops a live clock, which carries neither a reason nor an earlier moment', () => {
		expect(attendanceWrittenByHand([attendanceRow({ id: 'clocked' })])).toEqual([]);
	});

	test('keeps the reading order of the rows it was given', () => {
		const rows = [
			attendanceRow({ id: 'clocked' }),
			attendanceRow({ id: 'backdated', edit_reason: '대신 기록합니다' }),
			attendanceRow({ id: 'moved', original_occurred_at: '2026-08-31T23:00:00.000Z' })
		];
		expect(attendanceWrittenByHand(rows).map((row) => row.id)).toEqual(['backdated', 'moved']);
	});
});
