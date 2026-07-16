import type { AttendanceSummary } from './src/routes/attendance/attendance-context.svelte';
import { todayDateInTimeZone } from './src/routes/attendance/shared/attendance-date';
import { buildAttendanceAbsenceFixtures } from './dev-attendance-absence-fixtures';
import { devAttendanceLocations, devAttendancePeople, devAttendancePresences } from './dev-attendance-fixture-data';
import { buildAttendanceEventFixtures } from './dev-attendance-work-fixtures';

export function buildAttendanceSummaryFixture(month: string, currentTime: Date = new Date()): AttendanceSummary {
	const today = currentTime;
	const todayDate = todayDateInTimeZone('Asia/Seoul', today);
	const todayMonth = todayDate.slice(0, 7);
	const todayDay = Number(todayDate.slice(8, 10));
	const [yearString, monthString] = month.split('-');
	const yearNumber = Number(yearString);
	const monthNumber = Number(monthString);

	if (!yearNumber || !monthNumber) {
		return buildSummary(month, [], [], currentTime);
	}

	const lastDayInMonth = new Date(Date.UTC(yearNumber, monthNumber, 0)).getUTCDate();
	const endDay = resolveFixtureEndDay(month, todayMonth, todayDay, lastDayInMonth);
	const absences = buildAttendanceAbsenceFixtures({
		month,
		todayDate,
		todayMonth,
		lastDayInMonth,
		todayDay,
	});
	const events = buildAttendanceEventFixtures({
		month,
		todayDate,
		todayMonth,
		endDay,
		absences,
	});

	return buildSummary(month, events, absences, currentTime);
}

function resolveFixtureEndDay(month: string, todayMonth: string, todayDay: number, lastDayInMonth: number): number {
	if (month === todayMonth) return todayDay;
	if (month < todayMonth) return lastDayInMonth;
	return 0;
}

function buildSummary(
	month: string,
	events: AttendanceSummary['events'],
	absences: AttendanceSummary['absences'],
	serverTime: Date
): AttendanceSummary {
	return {
		month,
		serverTime: serverTime.toISOString(),
		timeZoneAuthoritative: true,
		currentUserEmail: 'kim@example.com',
		isAdmin: true,
		timeZone: 'Asia/Seoul',
		events,
		absences,
		members: devAttendancePeople.map((person) => ({
			email: person.email,
			displayName: person.name,
			mattermostUsername: person.mattermostUsername,
		})),
		todayStatus: '근무 중',
		locations: devAttendanceLocations,
		teamViewVisibleToAll: true,
		teamViewBlocked: false,
		presences: devAttendancePresences,
	};
}
