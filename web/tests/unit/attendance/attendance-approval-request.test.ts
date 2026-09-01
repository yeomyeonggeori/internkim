import { describe, expect, test } from 'bun:test';
import {
	attendanceApprovalDetailLines,
	attendanceApprovalKindLabel,
	type AttendanceApprovalKnownEvent
} from '../../../src/routes/attendance/approval/attendance-approval-summary';
import {
	attendanceApprovalRequestFrom,
	type AttendanceApprovalRow
} from '../../../src/routes/attendance/approval/attendance-approval-types';

const labels = {
	attendanceAdd: '기록 추가',
	attendanceEdit: '기록 수정',
	attendanceRemove: '기록 삭제',
	clockIn: '출근',
	clockOut: '퇴근'
};

const knownEvents: Record<string, AttendanceApprovalKnownEvent> = {
	'event-1': { localDate: '2026-07-01', localTime: '09:03:00', kind: 'clock_in' }
};

function findEvent(eventID: string): AttendanceApprovalKnownEvent | undefined {
	return knownEvents[eventID];
}

function rowWith(kind: string, payload: unknown): AttendanceApprovalRow {
	return {
		id: 'approval-1',
		member_id: 'member-1',
		asked_by: '이샘플',
		kind,
		payload,
		reason: '깜빡했습니다',
		created_at: '2026-07-15T01:00:00Z'
	};
}

describe('attendance approval request', () => {
	test('reads an addition request', () => {
		const request = attendanceApprovalRequestFrom(
			rowWith('attendance_add', {
				kind: 'clock_out',
				localDate: '2026-07-01',
				localTime: '18:30:00',
				location: '사무실'
			})
		);
		expect(request.askedBy).toBe('이샘플');
		expect(request.detail).toEqual({
			kind: 'attendance_add',
			addition: {
				attendanceKind: 'clock_out',
				localDate: '2026-07-01',
				localTime: '18:30',
				location: '사무실'
			}
		});
		expect(attendanceApprovalKindLabel(request.detail, labels)).toBe('기록 추가');
		expect(attendanceApprovalDetailLines(request.detail, labels, findEvent)).toEqual([
			'퇴근 2026-07-01 18:30 · 사무실'
		]);
	});

	test('reads a removal request and names the record it would remove', () => {
		const request = attendanceApprovalRequestFrom(
			rowWith('attendance_remove', { eventID: 'event-1' })
		);
		expect(request.detail).toEqual({ kind: 'attendance_remove', removedEventID: 'event-1' });
		expect(attendanceApprovalDetailLines(request.detail, labels, findEvent)).toEqual([
			'출근 2026-07-01 09:03'
		]);
	});

	test('reads a correction request in the wire shape the client sent', () => {
		const request = attendanceApprovalRequestFrom(
			rowWith('attendance_edit', {
				corrections: [
					{
						event_id: 'event-1',
						local_date: '2026-07-01',
						local_time: '10:00',
						location: '재택'
					}
				]
			})
		);
		expect(attendanceApprovalDetailLines(request.detail, labels, findEvent)).toEqual([
			'출근 2026-07-01 09:03 → 2026-07-01 10:00 · 재택'
		]);
	});

	test('says nothing about a removed record it cannot see rather than showing its id', () => {
		const request = attendanceApprovalRequestFrom(
			rowWith('attendance_remove', { eventID: 'event-elsewhere' })
		);
		expect(attendanceApprovalDetailLines(request.detail, labels, findEvent)).toEqual([]);
	});

	test('shows only the requested time when the corrected record is outside the loaded month', () => {
		const request = attendanceApprovalRequestFrom(
			rowWith('attendance_edit', {
				corrections: [
					{ event_id: 'event-elsewhere', local_date: '2026-05-02', local_time: '10:00', location: '' }
				]
			})
		);
		expect(attendanceApprovalDetailLines(request.detail, labels, findEvent)).toEqual([
			'2026-05-02 10:00'
		]);
	});

	test('refuses a kind nothing knows how to show', () => {
		expect(() => attendanceApprovalRequestFrom(rowWith('leave_request', {}))).toThrow();
	});
});
