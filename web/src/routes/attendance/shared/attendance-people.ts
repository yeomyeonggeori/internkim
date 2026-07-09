import type { AttendanceAbsence, AttendanceEvent, AttendancePresence } from '../attendance-context.svelte';
import { absencesForDate } from './attendance-absence';
import { isWeekend, todayDateInTimeZone } from './attendance-date';
import { computeDayEvents } from './attendance-day-events';
import type { AttendanceWorkSegment } from './attendance-work-segments';

export type PersonStatus = 'working' | 'finished' | 'absence' | 'absent' | 'weekend' | 'upcoming';

export type PersonToday = {
	email: string;
	displayName: string;
	image?: string;
	mattermostUsername: string;
	status: PersonStatus;
	clockIn?: AttendanceEvent;
	clockOut?: AttendanceEvent;
	segments: AttendanceWorkSegment[];
	activeSegment?: AttendanceWorkSegment;
	absence?: AttendanceAbsence;
	workedMinutes: number;
	locationName?: string;
	locationID?: string;
	presence?: AttendancePresence;
};

export function uniquePeople(
	events: AttendanceEvent[],
	absences: AttendanceAbsence[] = []
): Pick<PersonToday, 'email' | 'displayName' | 'image' | 'mattermostUsername'>[] {
	const map = new Map<string, Pick<PersonToday, 'email' | 'displayName' | 'image' | 'mattermostUsername'>>();
	for (const event of events) {
		if (!map.has(event.email)) {
			map.set(event.email, {
				email: event.email,
				displayName: event.displayName || event.mattermostUsername || event.email,
				mattermostUsername: event.mattermostUsername,
			});
		}
	}
	for (const absence of absences) {
		if (!map.has(absence.email)) {
			map.set(absence.email, {
				email: absence.email,
				displayName: absence.email,
				mattermostUsername: ''
			});
		}
	}
	return [...map.values()].sort((a, b) => a.displayName.localeCompare(b.displayName));
}

export function computePeopleToday(
	date: string,
	events: AttendanceEvent[],
	presences?: Record<string, AttendancePresence>,
	today: string = todayDateInTimeZone(),
	absences: AttendanceAbsence[] = []
): PersonToday[] {
	const activeAbsences = isWeekend(date) ? [] : absences.filter((absence) => absence.date === date && !absence.canceledAt);
	const people = uniquePeople(events, activeAbsences);
	const byPerson = new Map<string, AttendanceEvent[]>();
	for (const event of events) {
		if (event.canceledAt) continue;
		const list = byPerson.get(event.email) ?? [];
		list.push(event);
		byPerson.set(event.email, list);
	}
	const fallbackStatus: PersonStatus = isWeekend(date)
		? 'weekend'
		: date > today
			? 'upcoming'
			: 'absent';
	return people.map((person) => {
		const day = computeDayEvents(date, byPerson.get(person.email) ?? [], { currentDate: today });
		const absence = absencesForDate(activeAbsences, date, person.email)[0];
		let status: PersonStatus = fallbackStatus;
		if (day.inProgress) status = 'working';
		else if (day.segments.length > 0) status = 'finished';
		else if (day.clockIn) status = 'working';
		else if (absence) status = 'absence';
		return {
			...person,
			status,
			clockIn: day.clockIn,
			clockOut: day.clockOut,
			segments: day.segments,
			activeSegment: day.activeSegment,
			absence,
			workedMinutes: day.workedMinutes,
			locationID: day.activeSegment?.locationID ?? day.segments[0]?.locationID,
			locationName: day.activeSegment?.locationName ?? day.segments[0]?.locationName,
			presence: presences?.[person.email],
		};
	});
}

export function statusForDay(
	date: string,
	events: AttendanceEvent[],
	absences: AttendanceAbsence[] = [],
	today: string = todayDateInTimeZone()
): PersonStatus {
	const day = computeDayEvents(date, events, { currentDate: today });
	if (day.inProgress) return 'working';
	if (day.segments.length > 0) return 'finished';
	if (!isWeekend(date) && absences.some((absence) => absence.date === date && !absence.canceledAt)) return 'absence';
	return 'absent';
}
