import { describe, expect, test } from 'bun:test';
import { computeHeatmap } from '../../../src/routes/attendance/shared/attendance-heatmap';
import type { AttendanceAbsence, AttendanceEvent } from '../../../src/routes/attendance/attendance-context.svelte';

describe('attendance heatmap', () => {
	test('does not count outside-month continuation absences as current-month people', () => {
		const events: AttendanceEvent[] = [
			{
				id: 'event-1',
				mattermostUserID: 'user-1',
				mattermostUsername: 'kim',
				email: 'kim@example.com',
				displayName: '김철수',
				kind: 'clock_in',
				occurredAt: '2026-05-01T09:00:00+09:00',
				localDate: '2026-05-01',
				localTime: '09:00',
				timeZoneAtEvent: 'Asia/Seoul',
				source: 'test',
				resultPostID: '',
			},
		];
		const absences: AttendanceAbsence[] = [
			{
				id: 'absence-next-month',
				email: 'lee@example.com',
				kind: 'business_trip',
				labelKey: 'business_trip',
				date: '2026-06-01',
				createdAt: '2026-05-01T09:00:00+09:00',
			},
		];

		const firstDay = computeHeatmap('2026-05', events, absences)[0];

		expect(firstDay.totalPeople).toBe(1);
		expect(firstDay.absenceCount).toBe(0);
	});

	test('counts active absences by person for each date', () => {
		const events: AttendanceEvent[] = [
			{
				id: 'event-1',
				mattermostUserID: 'user-1',
				mattermostUsername: 'kim',
				email: 'kim@example.com',
				displayName: '김철수',
				kind: 'clock_in',
				occurredAt: '2026-05-01T09:00:00+09:00',
				localDate: '2026-05-01',
				localTime: '09:00',
				timeZoneAtEvent: 'Asia/Seoul',
				source: 'test',
				resultPostID: '',
			},
		];
		const absences: AttendanceAbsence[] = [
			attendanceAbsence('absence-1', 'lee@example.com', '2026-05-01'),
			attendanceAbsence('absence-2', 'lee@example.com', '2026-05-01'),
			attendanceAbsence('absence-3', 'park@example.com', '2026-05-01'),
			attendanceAbsence('absence-4', 'choi@example.com', '2026-05-01', '2026-05-01T10:00:00+09:00'),
		];

		const firstDay = computeHeatmap('2026-05', events, absences)[0];

		expect(firstDay.totalPeople).toBe(3);
		expect(firstDay.absenceCount).toBe(2);
	});
});

function attendanceAbsence(
	id: string,
	email: string,
	date: string,
	canceledAt?: string
): AttendanceAbsence {
	return {
		id,
		email,
		kind: 'leave',
		labelKey: 'leave',
		date,
		createdAt: `${date}T09:00:00+09:00`,
		canceledAt,
	};
}
