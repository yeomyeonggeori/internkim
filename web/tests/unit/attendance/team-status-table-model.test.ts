import { describe, expect, test } from 'bun:test';
import type {
	AttendanceAbsence,
	AttendanceEvent,
	AttendanceKind,
	AttendanceMember,
	AttendanceSummary
} from '../../../src/routes/attendance/attendance-context.svelte';
import { attendanceText } from '../../../src/routes/attendance/text';
import { dayWidthPercent } from '../../../src/routes/attendance/shared/day-timeline';
import { buildTeamStatusRows } from '../../../src/routes/attendance/team/team-status-table-model';

describe('team status table model', () => {
	test('maps monthly rows to location-aware statuses', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('kim-lab-in', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '09:00', 'lab-a', 'Lab A'),
				attendanceEvent('kim-lab-out', 'kim@example.com', '김철수', '2026-06-16', 'clock_out', '10:00', 'lab-a', 'Lab A'),
				attendanceEvent('kim-client-in', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '11:00', 'client-site', '고객사'),
				attendanceEvent('kim-client-out', 'kim@example.com', '김철수', '2026-06-16', 'clock_out', '12:00', 'client-site', '고객사'),
				attendanceEvent('kim-working-in', 'kim@example.com', '김철수', '2026-06-17', 'clock_in', '13:00', 'client-site', '고객사'),
				attendanceEvent('park-name', 'park@example.com', '박지민', '2026-06-16', 'clock_in', '09:00', 'lab-a', 'Lab A'),
				attendanceEvent('park-out', 'park@example.com', '박지민', '2026-06-16', 'clock_out', '10:00', 'lab-a', 'Lab A'),
			],
			[
				attendanceAbsence('lee-1', 'lee@example.com', 'other', '2026-06-17', {
					reason: 'client visit',
					createdBy: 'admin@example.com',
				}),
				attendanceAbsence('lee-2', 'lee@example.com', 'other', '2026-06-18'),
				attendanceAbsence('lee-3', 'lee@example.com', 'other', '2026-06-19'),
			]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-17', summary, '15:00');
		const kim = rows.find((row) => row.email === 'kim@example.com');
		const lee = rows.find((row) => row.email === 'lee@example.com');
		const park = rows.find((row) => row.email === 'park@example.com');

		expect(kim?.days.length).toBe(30);
		expect(kim).toMatchObject({
			currentLocationName: '고객사',
			currentLocationColor: '#f59e0b',
		});
		const labAWidthPercent = dayWidthPercent('09:00', '10:00');
		const clientSiteWidthPercent = dayWidthPercent('11:00', '12:00');
		expect(kim?.days.find((day) => day.date === '2026-06-16')).toMatchObject({
			label: '2시간',
			tone: 'finished',
			totalDurationLabel: '2시간',
			segments: [
				{
					locationName: 'Lab A',
					locationColor: '#22c55e',
					timeLabel: '09:00-10:00',
					durationLabel: '1시간',
					widthPercent: labAWidthPercent,
					tooltipLabel: 'Lab A 09:00-10:00 · 1시간',
				},
				{
					locationName: '고객사',
					locationColor: '#f59e0b',
					timeLabel: '11:00-12:00',
					durationLabel: '1시간',
					widthPercent: clientSiteWidthPercent,
					tooltipLabel: '고객사 11:00-12:00 · 1시간',
				},
			],
		});
		const openSegmentWidthPercent = dayWidthPercent('13:00', '15:00');
		expect(kim?.days.find((day) => day.date === '2026-06-17')).toMatchObject({
			label: '2시간',
			tone: 'working',
			locationName: '고객사',
			locationColor: '#f59e0b',
			detailLabel: '13:00',
			totalDurationLabel: '2시간',
			segments: [
				{
					locationName: '고객사',
					locationColor: '#f59e0b',
					timeLabel: '13:00~',
					durationLabel: '진행 중',
					widthPercent: openSegmentWidthPercent,
					tooltipLabel: '고객사 13:00~ · 진행 중',
					isOpen: true,
				},
			],
		});
		expect(lee?.days.find((day) => day.date === '2026-06-17')).toMatchObject({
			label: '기타',
			tone: 'absence',
			absenceTone: 'other',
			absenceDetail: {
				label: '기타',
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
			label: '-',
			tone: 'absent',
		});
	});

	test('uses registered location colors without interpreting location names', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('choi-room-in', 'choi@example.com', '최민준', '2026-06-16', 'clock_in', '09:00', '', '회의실 A'),
				attendanceEvent('jung-remote-in', 'jung@example.com', '정수아', '2026-06-16', 'clock_in', '09:00', '', 'remote'),
			],
			[]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-16', summary, '09:45');
		const choiDay = rows.find((row) => row.email === 'choi@example.com')?.days.find((day) => day.date === '2026-06-16');
		const jungDay = rows.find((row) => row.email === 'jung@example.com')?.days.find((day) => day.date === '2026-06-16');

		expect(choiDay).toMatchObject({
			label: '45분',
			tone: 'working',
			locationName: '회의실 A',
			locationColor: '#a855f7',
		});
		expect(jungDay).toMatchObject({
			label: '45분',
			tone: 'working',
			locationName: 'remote',
		});
		expect(jungDay?.locationColor).toBe(undefined);
	});

	test('maps monthly status rows across every date in the selected month', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '09:00'),
				attendanceEvent('kim-out', 'kim@example.com', '김철수', '2026-06-16', 'clock_out', '18:00'),
				attendanceEvent('park-name', 'park@example.com', '박지민', '2026-06-30', 'clock_in', '09:00'),
			],
			[
				attendanceAbsence('lee-1', 'lee@example.com', 'other', '2026-06-01'),
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
			label: '9시간',
			tone: 'finished',
		});
		expect(lee?.days.find((day) => day.date === '2026-06-01')).toMatchObject({
			label: '기타',
			tone: 'absence',
		});
		expect(choi?.days.find((day) => day.date === '2026-06-30')).toMatchObject({
			label: '휴가',
			tone: 'absence',
		});
	});

	test('shows registered members without selected month records', () => {
		const summary = attendanceSummaryWithMembers(
			[
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-16', 'clock_in', '09:00'),
			],
			[
				attendanceAbsence('july-1', 'july@example.com', 'leave', '2026-07-06', {
					rangeID: 'july-leave',
					startDate: '2026-07-06',
					endDate: '2026-07-06',
				}),
			],
			[
				attendanceMember('kim@example.com', '김철수', 'kim'),
				attendanceMember('park@example.com', '박지민', 'park'),
			]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-20');
		const park = rows.find((row) => row.email === 'park@example.com');

		expect(rows.map((row) => row.email)).toEqual(['kim@example.com', 'park@example.com']);
		expect(park?.days.find((day) => day.date === '2026-06-16')).toMatchObject({
			label: '-',
			tone: 'absent',
		});
	});

	test('does not show absence labels on weekends', () => {
		const summary = attendanceSummary(
			[
				attendanceEvent('kim-name', 'kim@example.com', '김철수', '2026-06-05', 'clock_in', '09:00'),
				attendanceEvent('kim-out', 'kim@example.com', '김철수', '2026-06-05', 'clock_out', '18:00'),
			],
			[
				attendanceAbsence('kim-weekend-leave', 'kim@example.com', 'leave', '2026-06-06'),
			]
		);

		const rows = buildTeamStatusRows('2026-06', summary, attendanceText.ko, '2026-06-06');
		const kimWeekend = rows.find((row) => row.email === 'kim@example.com')?.days.find((day) => day.date === '2026-06-06');

		expect(kimWeekend).toMatchObject({
			label: '-',
			tone: 'empty',
		});
	});

	test('uses current month summary for the current location while viewing another month', () => {
		const selectedSummary = attendanceSummaryForMonth('2026-05',
			[
				attendanceEvent('kim-may-name', 'kim@example.com', '김철수', '2026-05-20', 'clock_in', '09:00'),
				attendanceEvent('kim-may-out', 'kim@example.com', '김철수', '2026-05-20', 'clock_out', '18:00'),
			],
			[]
		);
		const currentSummary = attendanceSummaryForMonth('2026-06',
			[
				attendanceEvent('kim-current-in', 'kim@example.com', '김철수', '2026-06-24', 'clock_in', '10:00', 'client-site', '고객사'),
			],
			[]
		);

		const rows = buildTeamStatusRows('2026-05', selectedSummary, attendanceText.ko, '2026-06-24', currentSummary);
		const kim = rows.find((row) => row.email === 'kim@example.com');

		expect(kim).toMatchObject({
			currentLocationName: '고객사',
			currentLocationColor: '#f59e0b',
		});
		expect(kim?.days.find((day) => day.date === '2026-05-20')).toMatchObject({
			label: '9시간',
			tone: 'finished',
		});
	});
});

function attendanceSummary(events: AttendanceEvent[], absences: AttendanceAbsence[]): AttendanceSummary {
	return attendanceSummaryForMonth('2026-06', events, absences);
}

function attendanceSummaryWithMembers(
	events: AttendanceEvent[],
	absences: AttendanceAbsence[],
	members: AttendanceMember[]
): AttendanceSummary {
	return {
		...attendanceSummary(events, absences),
		members,
	};
}

function attendanceSummaryForMonth(month: string, events: AttendanceEvent[], absences: AttendanceAbsence[]): AttendanceSummary {
	return {
		month,
		currentUserEmail: 'kim@example.com',
		isAdmin: true,
		timeZone: 'Asia/Seoul',
		events,
		absences,
		members: attendanceMembersFromRecords(events, absences),
		todayStatus: '근무 중',
		locations: [
			{ id: 'lab-a', name: 'Lab A', color: '#22c55e', isDefault: true },
			{ id: 'client-site', name: '고객사', color: '#f59e0b', isDefault: false },
			{ id: 'meeting-room', name: '회의실 A', color: '#a855f7', isDefault: false },
		],
		teamViewVisibleToAll: true,
		teamViewBlocked: false,
		presences: {},
	};
}

function attendanceMember(email: string, displayName: string, mattermostUsername: string): AttendanceMember {
	return { email, displayName, mattermostUsername };
}

function attendanceMembersFromRecords(events: AttendanceEvent[], absences: AttendanceAbsence[]): AttendanceMember[] {
	const members = new Map<string, AttendanceMember>();
	for (const event of events) {
		if (members.has(event.email)) continue;
		members.set(event.email, attendanceMember(
			event.email,
			event.displayName || event.mattermostUsername || event.email,
			event.mattermostUsername
		));
	}
	for (const absence of absences) {
		if (members.has(absence.email)) continue;
		members.set(absence.email, attendanceMember(absence.email, absence.email, ''));
	}
	return [...members.values()].sort((first, second) => first.displayName.localeCompare(second.displayName));
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
