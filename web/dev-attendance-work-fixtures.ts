import type {
	AttendanceAbsence,
	AttendanceKind,
	AttendanceSummary,
} from './src/routes/attendance/attendance-context.svelte';
import { annotateClockIn, annotateClockOut, scenarioFor, type Annotation } from './dev-attendance-event-annotations';
import {
	devAttendanceLocations,
	devAttendancePeople,
	type DevAttendanceLocation,
	type DevAttendancePerson,
} from './dev-attendance-fixture-data';
import {
	devPopupOverflowAttendanceRows,
	devPopupOverflowDate,
	devPopupOverflowDay,
	devPopupOverflowEmail,
	devPopupOverflowMonth
} from './dev-popup-overflow-fixture';

type BuildAttendanceEventFixtureOptions = {
	month: string;
	todayDate: string;
	todayMonth: string;
	endDay: number;
	absences: AttendanceAbsence[];
};

export function buildAttendanceEventFixtures(options: BuildAttendanceEventFixtureOptions): AttendanceSummary['events'] {
	const { month, todayDate, todayMonth, endDay, absences } = options;
	const events: AttendanceSummary['events'] = [];
	let idSeed = 0;

	for (let day = 1; day <= endDay; day += 1) {
		const date = `${month}-${pad(day)}`;
		if (isWeekendDate(date)) continue;
		appendDayEvents(events, absences, date, todayDate, day, idSeed);
		idSeed = events.length;
	}

	idSeed = appendPopupOverflowScenario(events, idSeed, month, endDay);
	idSeed = appendCompletedThreeLocationScenario(events, idSeed, month, endDay);
	appendTodayMultipleLocationScenario(events, idSeed, month, todayMonth, todayDate);

	return events;
}

function appendDayEvents(
	events: AttendanceSummary['events'],
	absences: AttendanceAbsence[],
	date: string,
	todayDate: string,
	day: number,
	startIDSeed: number
): void {
	let idSeed = startIDSeed;
	devAttendancePeople.forEach((person, personIndex) => {
		if (absences.some((absence) => absence.date === date && absence.email === person.email)) return;
		if (isGeneratedAbsentPerson(person.email, day)) return;

		const location = pickLocation(personIndex, day);
		const clockInTime = generatedClockInTime(person, personIndex, day);
		const scenario = scenarioFor(day, personIndex);
		const messageSeed = day + personIndex;

		idSeed += 1;
		events.push(buildFixtureEvent({
			id: `in-${idSeed}`,
			person,
			date,
			kind: 'clock_in',
			localTime: clockInTime,
			location,
			annotation: annotateClockIn(scenario, location.id, messageSeed, date),
			idSeed,
		}));

		if (date === todayDate && personIndex % 2 === 0) return;

		if (day % 9 === 0 && personIndex === 2) {
			idSeed += 1;
			events.push(buildCanceledDuplicateClickEvent(person, date, clockInTime, location, idSeed));
		}

		const clockOutTime = generatedClockOutTime(personIndex, day);
		idSeed += 1;
		events.push(buildFixtureEvent({
			id: `out-${idSeed}`,
			person,
			date,
			kind: 'clock_out',
			localTime: clockOutTime,
			location,
			annotation: annotateClockOut(scenario, location.id, messageSeed + 1, date),
			idSeed,
		}));
	});
}

function appendPopupOverflowScenario(
	events: AttendanceSummary['events'],
	startIDSeed: number,
	month: string,
	endDay: number
): number {
	if (month !== devPopupOverflowMonth) return startIDSeed;
	if (endDay < devPopupOverflowDay) return startIDSeed;
	const person = devAttendancePeople.find((candidate) => candidate.email === devPopupOverflowEmail) ?? devAttendancePeople[0];
	removePersonDateEvents(events, person.email, devPopupOverflowDate);
	const rows: [AttendanceKind, string, string, string][] = devPopupOverflowAttendanceRows.map((row) => [
		row.kind,
		row.localTime,
		row.locationID,
		row.sourceMessage
	]);
	return appendMultiLocationEvents(events, startIDSeed, person, devPopupOverflowDate, rows);
}

function appendCompletedThreeLocationScenario(
	events: AttendanceSummary['events'],
	startIDSeed: number,
	month: string,
	endDay: number
): number {
	const sampleDay = 19;
	if (endDay < sampleDay) return startIDSeed;
	const date = `${month}-${pad(sampleDay)}`;
	const person = devAttendancePeople[0];
	removePersonDateEvents(events, person.email, date);
	return appendMultiLocationEvents(events, startIDSeed, person, date, [
		['clock_in', '08:30', 'remote', '오전 재택 시작'],
		['clock_out', '10:20', 'remote', '재택 마치고 사무실 이동'],
		['clock_in', '10:45', 'office', '사무실 도착'],
		['clock_out', '12:20', 'office', '외부 일정 이동'],
		['clock_in', '13:00', 'outside', '외부 일정 시작'],
		['clock_out', '17:30', 'outside', '외부 일정 종료'],
	]);
}

function appendTodayMultipleLocationScenario(
	events: AttendanceSummary['events'],
	startIDSeed: number,
	month: string,
	todayMonth: string,
	todayDate: string
): number {
	if (month !== todayMonth) return startIDSeed;
	const person = devAttendancePeople[0];
	removePersonDateEvents(events, person.email, todayDate);
	return appendMultiLocationEvents(events, startIDSeed, person, todayDate, [
		['clock_in', '08:30', 'remote', '오늘 재택할게요'],
		['clock_out', '10:20', 'remote', '재택 마치고 이동합니다'],
		['clock_in', '10:45', 'office', '사무실 도착했어요'],
		['clock_out', '12:20', 'office', '외부 일정으로 이동합니다'],
		['clock_in', '12:45', 'outside', '외부 일정 시작'],
	]);
}

function appendMultiLocationEvents(
	events: AttendanceSummary['events'],
	startIDSeed: number,
	person: DevAttendancePerson,
	date: string,
	rows: [AttendanceKind, string, string, string][]
): number {
	let idSeed = startIDSeed;
	for (const [kind, localTime, locationID, sourceMessage] of rows) {
		idSeed += 1;
		const location = findLocation(locationID);
		events.push(buildFixtureEvent({
			id: `multi-${idSeed}`,
			person,
			date,
			kind,
			localTime,
			location,
			annotation: {
				sourceMessage,
				confidence: 0.96,
				parsedAs: { kind, locationID: location.id },
			},
			idSeed,
		}));
	}
	return idSeed;
}

function buildFixtureEvent(options: {
	id: string;
	person: DevAttendancePerson;
	date: string;
	kind: AttendanceKind;
	localTime: string;
	location: DevAttendanceLocation;
	annotation: Annotation;
	idSeed: number;
}): AttendanceSummary['events'][number] {
	const { id, person, date, kind, localTime, location, annotation, idSeed } = options;
	return {
		id,
		mattermostUserID: person.mattermostUsername,
		mattermostUsername: person.mattermostUsername,
		email: person.email,
		displayName: person.name,
		kind,
		occurredAt: `${date}T${localTime}:00+09:00`,
		localDate: date,
		localTime,
		timeZoneAtEvent: 'Asia/Seoul',
		source: 'mattermost_button',
		resultPostID: `post-${idSeed}`,
		locationID: location.id,
		locationName: location.name,
		...annotation,
	};
}

function buildCanceledDuplicateClickEvent(
	person: DevAttendancePerson,
	date: string,
	clockInTime: string,
	location: DevAttendanceLocation,
	idSeed: number
): AttendanceSummary['events'][number] {
	const [hour, minute] = clockInTime.split(':').map(Number);
	const duplicateMinute = ((minute ?? 0) + 15) % 60;
	const canceledMinute = ((minute ?? 0) + 20) % 60;
	const duplicateTime = `${pad(hour ?? 0)}:${pad(duplicateMinute)}`;
	return {
		...buildFixtureEvent({
			id: `in-${idSeed}`,
			person,
			date,
			kind: 'clock_in',
			localTime: duplicateTime,
			location,
			annotation: {},
			idSeed,
		}),
		canceledAt: `${date}T${pad(hour ?? 0)}:${pad(canceledMinute)}:00+09:00`,
		cancelReason: 'duplicate_click',
	};
}

function removePersonDateEvents(events: AttendanceSummary['events'], email: string, date: string): void {
	for (let index = events.length - 1; index >= 0; index -= 1) {
		const event = events[index];
		if (event.email === email && event.localDate === date) events.splice(index, 1);
	}
}

function generatedClockInTime(person: DevAttendancePerson, personIndex: number, day: number): string {
	const hourJitter = (day + personIndex) % 3 === 0 ? 1 : 0;
	const minuteJitter = day * 13 + personIndex * 7;
	const minute = (person.baseMinute + minuteJitter) % 60;
	const hour = person.baseHour + hourJitter + Math.floor((person.baseMinute + minuteJitter) / 60);
	return `${pad(hour)}:${pad(minute)}`;
}

function generatedClockOutTime(personIndex: number, day: number): string {
	const outHour = 17 + (personIndex % 3);
	const outMinute = (personIndex * 11 + day * 3) % 60;
	return `${pad(outHour)}:${pad(outMinute)}`;
}

function isGeneratedAbsentPerson(email: string, day: number): boolean {
	if (email === 'jung@example.com' && day % 3 === 0) return true;
	return email === 'kang@example.com' && day % 4 === 0;
}

function pickLocation(personIndex: number, day: number): DevAttendanceLocation {
	if (day % 11 === 2 && personIndex === 0) return findLocation('bss');
	if (day % 7 === 3 && personIndex >= 4) return findLocation('outside');
	if (day % 5 === 0 && personIndex !== 0) return findLocation('remote');
	return findLocation('office');
}

function findLocation(locationID: string): DevAttendanceLocation {
	return devAttendanceLocations.find((location) => location.id === locationID) ?? devAttendanceLocations[0];
}

function isWeekendDate(date: string): boolean {
	const weekday = new Date(`${date}T00:00:00Z`).getUTCDay();
	return weekday === 0 || weekday === 6;
}

function pad(value: number): string {
	return value.toString().padStart(2, '0');
}
