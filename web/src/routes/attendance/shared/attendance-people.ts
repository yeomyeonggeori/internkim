import type { AttendanceEvent, AttendancePresence } from '../attendance-context.svelte';
import { isWeekend, todayDateInTimeZone } from './attendance-date';
import { computeDayEvents } from './attendance-day-events';

export type PersonStatus = 'working' | 'finished' | 'absent' | 'weekend' | 'upcoming';

export type PersonToday = {
	email: string;
	displayName: string;
	mattermostUsername: string;
	status: PersonStatus;
	clockIn?: AttendanceEvent;
	clockOut?: AttendanceEvent;
	workedMinutes: number;
	locationName?: string;
	locationID?: string;
	presence?: AttendancePresence;
};

export function uniquePeople(events: AttendanceEvent[]): Pick<PersonToday, 'email' | 'displayName' | 'mattermostUsername'>[] {
	const map = new Map<string, Pick<PersonToday, 'email' | 'displayName' | 'mattermostUsername'>>();
	for (const event of events) {
		if (!map.has(event.email)) {
			map.set(event.email, {
				email: event.email,
				displayName: event.displayName || event.mattermostUsername || event.email,
				mattermostUsername: event.mattermostUsername,
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
): PersonToday[] {
	const people = uniquePeople(events);
	const byPerson = new Map<string, AttendanceEvent[]>();
	for (const event of events) {
		if (event.localDate !== date) continue;
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
		const day = computeDayEvents(date, byPerson.get(person.email) ?? []);
		let status: PersonStatus = fallbackStatus;
		if (day.inProgress) status = 'working';
		else if (day.clockIn && day.clockOut) status = 'finished';
		else if (day.clockIn) status = 'working';
		return {
			...person,
			status,
			clockIn: day.clockIn,
			clockOut: day.clockOut,
			workedMinutes: day.workedMinutes,
			locationID: day.clockIn?.locationID,
			locationName: day.clockIn?.locationName,
			presence: presences?.[person.email],
		};
	});
}

export function statusForDay(date: string, events: AttendanceEvent[]): PersonStatus {
	const day = computeDayEvents(date, events.filter((event) => event.localDate === date));
	if (day.inProgress) return 'working';
	if (day.clockIn && day.clockOut) return 'finished';
	return 'absent';
}
