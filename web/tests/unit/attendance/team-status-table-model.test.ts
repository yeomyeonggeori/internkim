import { describe, expect, test } from 'bun:test';
import type {
	AttendanceAbsence,
	AttendanceEvent,
	AttendanceKind,
	AttendanceSummary
} from '../../../src/routes/attendance/attendance-context.svelte';
import { attendanceText } from '../../../src/routes/attendance/text';
import { buildTeamStatusRows } from '../../../src/routes/attendance/team/team-status-table-model';

describe('team status table model', () => {
	test('maps monthly rows to location-aware statuses', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('kim-office-in', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '09:00', 'office', '사무실'),
				attendanceEvent('kim-office-out', 'kim@example.com', '김철수', '2026-06-16', 'clock_out', '10:00', 'office', '사무실'),
				attendanceEvent('kim-remote-in', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '11:00', 'remote', '재택'),
				attendanceEvent('kim-remote-out', 'kim@example.com', '김철수', '2026-06-16', 'clock_out', '12:00', 'remote', '재택'),
				attendanceEvent('kim-working-in', 'kim@example.com', '김철수', '2026-06-17', 'clock_in', '13:00', 'remote', '재택'),
				attendanceEvent('park-name', 'park@example.com', '박지민', '2026-06-16', 'clock_in', '09:00', 'office', '사무실'),
			],
			[
				attendanceAbsence('lee-1', 'lee@example.com', 'business_trip', '2026-06-17', {
					reason: 'client visit',
					createdBy: 'admin@example.com',
				}),
				attendanceAbsence('lee-2', 'lee@example.com', 'other', '2026-06-18'),
				attendanceAbsence('lee-3', 'lee@example.com', 'other', '2026-06-19'),
			]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-20');
		const kim = rows.find((row) => row.email === 'kim@example.com');
		const lee = rows.find((row) => row.email === 'lee@example.com');
		const park = rows.find((row) => row.email === 'park@example.com');

		expect(kim?.days.length).toBe(30);
		expect(kim?.days.find((day) => day.date === '2026-06-16')).toMatchObject({
			label: '퇴근',
			tone: 'finished',
			detailLabel: '2h',
			totalDurationLabel: '2h',
			segments: [
				{ locationName: '사무실', locationColor: '#22c55e', timeLabel: '09:00-10:00', durationLabel: '1h' },
				{ locationName: '재택', locationColor: '#3b82f6', timeLabel: '11:00-12:00', durationLabel: '1h' },
			],
		});
		expect(kim?.days.find((day) => day.date === '2026-06-17')).toMatchObject({
			label: '재택',
			tone: 'working',
			locationName: '재택',
			locationColor: '#3b82f6',
			detailLabel: '13:00',
			segments: [{ locationName: '재택', locationColor: '#3b82f6', timeLabel: '13:00~', durationLabel: '진행 중', isOpen: true }],
		});
		expect(lee?.days.find((day) => day.date === '2026-06-17')).toMatchObject({
			label: '출장',
			tone: 'absence',
			absenceTone: 'work',
			absenceDetail: {
				label: '출장',
				periodLabel: '6/17',
				reason: 'client visit',
				createdBy: 'admin@example.com',
			},
		});
		expect(lee?.days.find((day) => day.date === '2026-06-18')).toMatchObject({
			label: '기타',
			tone: 'absence',
			absenceTone: 'other',
		});
		expect(lee?.days.find((day) => day.date === '2026-06-19')).toMatchObject({
			label: '기타',
			tone: 'absence',
			absenceTone: 'other',
		});
		expect(park?.days.find((day) => day.date === '2026-06-17')).toMatchObject({
			label: '미출근',
			tone: 'absent',
		});
	});

	test('maps monthly status rows across every date in the selected month', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '09:00'),
				attendanceEvent('kim-out', 'kim@example.com', '김철수', '2026-06-16', 'clock_out', '18:00'),
				attendanceEvent('park-name', 'park@example.com', '박지민', '2026-06-30', 'clock_in', '09:00'),
			],
			[
				attendanceAbsence('lee-1', 'lee@example.com', 'business_trip', '2026-06-01'),
				attendanceAbsence('choi-1', 'choi@example.com', 'leave', '2026-06-30'),
			]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-20');
		const kim = rows.find((row) => row.email === 'kim@example.com');
		const lee = rows.find((row) => row.email === 'lee@example.com');
		const choi = rows.find((row) => row.email === 'choi@example.com');

		expect(rows.map((row) => row.email)).toEqual([
			'choi@example.com',
			'lee@example.com',
			'kim@example.com',
			'park@example.com',
		]);
		expect(kim?.days.length).toBe(30);
		expect(kim?.days[0]?.date).toBe('2026-06-01');
		expect(kim?.days[29]?.date).toBe('2026-06-30');
		expect(kim?.days.find((day) => day.date === '2026-06-16')).toMatchObject({
			label: '퇴근',
			tone: 'finished',
		});
		expect(lee?.days.find((day) => day.date === '2026-06-01')).toMatchObject({
			label: '출장',
			tone: 'absence',
		});
		expect(choi?.days.find((day) => day.date === '2026-06-30')).toMatchObject({
			label: '휴가',
			tone: 'absence',
		});
	});

	test('excludes people who only have records outside the selected month', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '09:00'),
			],
			[
				attendanceAbsence('july-1', 'july@example.com', 'leave', '2026-07-06', {
					rangeID: 'july-leave',
					startDate: '2026-07-06',
					endDate: '2026-07-06',
				}),
			]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-20');

		expect(rows.map((row) => row.email)).toEqual(['kim@example.com']);
	});
});

function attendanceSummary(events: AttendanceEvent[], absences: AttendanceAbsence[]): AttendanceSummary {
	return {
		month: '2026-06',
		currentUserEmail: 'kim@example.com',
		isAdmin: true,
		timeZone: 'Asia/Seoul',
		events,
		absences,
		todayStatus: '근무 중',
		locations: [
			{ id: 'office', name: '사무실', color: '#22c55e', isDefault: true },
			{ id: 'remote', name: '재택', color: '#3b82f6', isDefault: false },
		],
		teamViewVisibleToAll: true,
		teamViewBlocked: false,
		presences: {},
	};
}

function attendanceEvent(
	id: string,
	email: string,
	displayName: string,
	localDate: string,
	kind: AttendanceKind,
	localTime: string,
	locationID = '',
	locationName = ''
): AttendanceEvent {
	return {
		id,
		mattermostUserID: email,
		mattermostUsername: email.split('@')[0] ?? email,
		email,
		displayName,
		kind,
		occurredAt: `${localDate}T${localTime}:00+09:00`,
		localDate,
		localTime,
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'test',
		resultPostID: `${id}-post`,
		locationID,
		locationName,
	};
}

function attendanceAbsence(
	id: string,
	email: string,
	kind: AttendanceAbsence['kind'],
	date: string,
	overrides: Partial<AttendanceAbsence> = {}
): AttendanceAbsence {
	return {
		id,
		email,
		kind,
		labelKey: kind,
		date,
		createdAt: `${date}T09:00:00+09:00`,
		...overrides,
	};
}
