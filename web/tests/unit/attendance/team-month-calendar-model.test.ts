import { describe, expect, test } from 'bun:test';
import type {
	AttendanceAbsence,
	AttendanceEvent,
	AttendanceKind,
	AttendanceSummary
} from '../../../src/routes/attendance/attendance-context.svelte';
import type { DayHeatCell } from '../../../src/routes/attendance/shared/attendance-aggregation';
import { eachDayOfMonth } from '../../../src/routes/attendance/shared/attendance-date';
import { attendanceText } from '../../../src/routes/attendance/text';
import {
	buildTeamAbsenceByDate,
	buildTeamCalendarCells
} from '../../../src/routes/attendance/team/team-month-calendar-model';

describe('team month calendar model', () => {
	test('builds a calendar grid with only the weeks needed for the month', () => {
		const cells = buildTeamCalendarCells(heatCells('2026-06'), '2026-06');
		const sixWeekCells = buildTeamCalendarCells(heatCells('2026-08'), '2026-08');

		expect(cells.length).toBe(35);
		expect(cells[0]).toMatchObject({ date: '2026-05-31', inCurrentMonth: false });
		expect(cells[1]).toMatchObject({ date: '2026-06-01', inCurrentMonth: true });
		expect(cells[30]).toMatchObject({ date: '2026-06-30', inCurrentMonth: true });
		expect(cells[31]).toMatchObject({ date: '2026-07-01', inCurrentMonth: false });
		expect(cells[34]).toMatchObject({ date: '2026-07-04', inCurrentMonth: false });
		expect(sixWeekCells.length).toBe(42);
		expect(sixWeekCells[0]).toMatchObject({ date: '2026-07-26', inCurrentMonth: false });
		expect(sixWeekCells[41]).toMatchObject({ date: '2026-09-05', inCurrentMonth: false });
	});

	test('groups consecutive absences into visual spans', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('lee-name', 'lee@example.com', '이영희', '2026-06-01', 'clock_in', '09:00'),
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-01', 'clock_in', '09:00'),
			],
			[
				attendanceAbsence('lee-1', 'lee@example.com', 'other', '2026-06-02'),
				attendanceAbsence('lee-2', 'lee@example.com', 'other', '2026-06-03'),
				attendanceAbsence('kim-1', 'kim@example.com', 'leave', '2026-06-02'),
			]
		);

		const absencesByDate = buildTeamAbsenceByDate(summary, attendanceText.ko);
		const leeFirstDay = absencesByDate.get('2026-06-02')?.find((absence) => absence.label === '이영희 기타');
		const leeSecondDay = absencesByDate.get('2026-06-03')?.find((absence) => absence.label === '이영희 기타');
		const kimDay = absencesByDate.get('2026-06-02')?.find((absence) => absence.label === '김철수 휴가');

		expect(leeFirstDay).toMatchObject({ tone: 'other', periodLabel: '6/2', starts: true, ends: false });
		expect(leeSecondDay).toMatchObject({ tone: 'other', periodLabel: '6/3', starts: false, ends: true });
		expect(kimDay).toMatchObject({ tone: 'leave', periodLabel: '6/2', starts: true, ends: true });
	});

	test('sorts overlapping absences by visible length and creation order', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('park-name', 'park@example.com', '박지민', '2026-06-01', 'clock_in', '09:00'),
				attendanceEvent('lee-name', 'lee@example.com', '이영희', '2026-06-01', 'clock_in', '09:00'),
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-01', 'clock_in', '09:00'),
			],
			[
				attendanceAbsence('park-1', 'park@example.com', 'other', '2026-06-09', {
					rangeID: 'park-other',
					startDate: '2026-06-09',
					endDate: '2026-06-11',
					createdAt: '2026-06-01T09:00:00+09:00',
				}),
				attendanceAbsence('park-2', 'park@example.com', 'other', '2026-06-10', {
					rangeID: 'park-other',
					startDate: '2026-06-09',
					endDate: '2026-06-11',
					createdAt: '2026-06-01T09:00:00+09:00',
				}),
				attendanceAbsence('park-3', 'park@example.com', 'other', '2026-06-11', {
					rangeID: 'park-other',
					startDate: '2026-06-09',
					endDate: '2026-06-11',
					createdAt: '2026-06-01T09:00:00+09:00',
				}),
				attendanceAbsence('kim-1', 'kim@example.com', 'leave', '2026-06-10', {
					createdAt: '2026-06-02T09:00:00+09:00',
				}),
				attendanceAbsence('lee-1', 'lee@example.com', 'other', '2026-06-10', {
					createdAt: '2026-06-01T10:00:00+09:00',
				}),
			]
		);

		const absencesByDate = buildTeamAbsenceByDate(summary, attendanceText.ko);
		const labels = absencesByDate.get('2026-06-10')?.map((absence) => absence.label);
		const lanes = absencesByDate.get('2026-06-10')?.map((absence) => absence.lane);
		const visibility = absencesByDate.get('2026-06-10')?.map((absence) => absence.isVisible);
		const parkOther = absencesByDate.get('2026-06-10')?.find((absence) => absence.label === '박지민 기타');

		expect(labels).toEqual(['박지민 기타', '이영희 기타', '김철수 휴가']);
		expect(lanes).toEqual([0, 1, 2]);
		expect(visibility).toEqual([true, true, false]);
		expect(parkOther?.periodLabel).toBe('6/9~6/11');
	});

	test('keeps schedule lanes stable and hides overflow ranges across all visible days', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('park-name', 'park@example.com', '박지민', '2026-06-01', 'clock_in', '09:00'),
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-01', 'clock_in', '09:00'),
				attendanceEvent('lee-name', 'lee@example.com', '이영희', '2026-06-01', 'clock_in', '09:00'),
			],
			[
				rangeAbsence('park-18', 'park@example.com', 'other', 'park-other', '2026-06-18', '2026-06-18', '2026-06-23'),
				rangeAbsence('park-19', 'park@example.com', 'other', 'park-other', '2026-06-19', '2026-06-18', '2026-06-23'),
				rangeAbsence('park-22', 'park@example.com', 'other', 'park-other', '2026-06-22', '2026-06-18', '2026-06-23'),
				rangeAbsence('park-23', 'park@example.com', 'other', 'park-other', '2026-06-23', '2026-06-18', '2026-06-23'),
				rangeAbsence('kim-18', 'kim@example.com', 'leave', 'kim-leave', '2026-06-18', '2026-06-18', '2026-06-23', {
					createdAt: '2026-06-01T10:00:00+09:00',
				}),
				rangeAbsence('kim-19', 'kim@example.com', 'leave', 'kim-leave', '2026-06-19', '2026-06-18', '2026-06-23', {
					createdAt: '2026-06-01T10:00:00+09:00',
				}),
				rangeAbsence('kim-22', 'kim@example.com', 'leave', 'kim-leave', '2026-06-22', '2026-06-18', '2026-06-23', {
					createdAt: '2026-06-01T10:00:00+09:00',
				}),
				rangeAbsence('kim-23', 'kim@example.com', 'leave', 'kim-leave', '2026-06-23', '2026-06-18', '2026-06-23', {
					createdAt: '2026-06-01T10:00:00+09:00',
				}),
				rangeAbsence('lee-22', 'lee@example.com', 'other', 'lee-other', '2026-06-22', '2026-06-22', '2026-06-24'),
				rangeAbsence('lee-23', 'lee@example.com', 'other', 'lee-other', '2026-06-23', '2026-06-22', '2026-06-24'),
				rangeAbsence('lee-24', 'lee@example.com', 'other', 'lee-other', '2026-06-24', '2026-06-22', '2026-06-24'),
			]
		);

		const absencesByDate = buildTeamAbsenceByDate(summary, attendanceText.ko);
		const parkFirstChunk = absencesByDate.get('2026-06-18')?.find((absence) => absence.label === '박지민 기타');
		const parkSecondChunk = absencesByDate.get('2026-06-22')?.find((absence) => absence.label === '박지민 기타');
		const leeHiddenDays = ['2026-06-22', '2026-06-23', '2026-06-24'].map((date) =>
			absencesByDate.get(date)?.find((absence) => absence.label === '이영희 기타')
		);

		expect(parkFirstChunk).toMatchObject({ lane: 0, isVisible: true, starts: true, ends: false });
		expect(parkSecondChunk).toMatchObject({ lane: 0, isVisible: true, starts: true, ends: false });
		expect(leeHiddenDays.map((absence) => absence?.lane)).toEqual([2, 2, 2]);
		expect(leeHiddenDays.map((absence) => absence?.isVisible)).toEqual([false, false, false]);
	});
});

function heatCells(month: string): DayHeatCell[] {
	return eachDayOfMonth(month).map((date) => ({
		date,
		presentCount: date.endsWith('-01') ? 1 : 0,
		totalPeople: 2,
		level: date.endsWith('-01') ? 1 : 0,
	}));
}

function attendanceSummary(events: AttendanceEvent[], absences: AttendanceAbsence[]): AttendanceSummary {
	return {
		month: '2026-06',
		currentUserEmail: 'kim@example.com',
		isAdmin: true,
		timeZone: 'Asia/Seoul',
		events,
		absences,
		members: [],
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

function rangeAbsence(
	id: string,
	email: string,
	kind: AttendanceAbsence['kind'],
	rangeID: string,
	date: string,
	startDate: string,
	endDate: string,
	overrides: Partial<AttendanceAbsence> = {}
): AttendanceAbsence {
	return attendanceAbsence(id, email, kind, date, {
		rangeID,
		startDate,
		endDate,
		createdAt: '2026-06-01T09:00:00+09:00',
		isChunkStart: date === startDate || date.endsWith('-22'),
		isChunkEnd: date.endsWith('-19') || date === endDate,
		...overrides,
	});
}
