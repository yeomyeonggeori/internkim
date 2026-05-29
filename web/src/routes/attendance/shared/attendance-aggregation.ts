import type { AttendanceEvent, AttendancePresence } from '../attendance-context.svelte';
import { eachDayOfMonth, isWeekday, isWeekend, isoWeekStart, todayDateInTimeZone } from './attendance-date';
import { formatTimeOfDay } from './attendance-format';

export type DayEvents = {
	date: string;
	events: AttendanceEvent[];
	clockIn?: AttendanceEvent;
	clockOut?: AttendanceEvent;
	workedMinutes: number;
	inProgress: boolean;
};

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

export function groupEventsByDay(events: AttendanceEvent[]): Map<string, AttendanceEvent[]> {
	const map = new Map<string, AttendanceEvent[]>();
	for (const event of events) {
		if (event.canceledAt) continue;
		const list = map.get(event.localDate) ?? [];
		list.push(event);
		map.set(event.localDate, list);
	}
	for (const list of map.values()) {
		list.sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	}
	return map;
}

export function computeDayEvents(date: string, events: AttendanceEvent[]): DayEvents {
	const sorted = [...events].sort((a, b) => a.occurredAt.localeCompare(b.occurredAt));
	const clockIn = sorted.find((event) => event.kind === 'clock_in');
	const clockOut = [...sorted].reverse().find((event) => event.kind === 'clock_out');
	const workedMinutes = clockIn && clockOut ? minutesBetween(clockIn.occurredAt, clockOut.occurredAt) : 0;
	const inProgress = !!clockIn && !clockOut;
	return { date, events: sorted, clockIn, clockOut, workedMinutes, inProgress };
}

export function minutesBetween(start: string, end: string): number {
	const startMs = new Date(start).getTime();
	const endMs = new Date(end).getTime();
	if (Number.isNaN(startMs) || Number.isNaN(endMs) || endMs <= startMs) return 0;
	return Math.round((endMs - startMs) / 60000);
}

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

export type DayHeatCell = {
	date: string;
	presentCount: number;
	totalPeople: number;
	level: 0 | 1 | 2 | 3;
};

export function computeHeatmap(month: string, events: AttendanceEvent[]): DayHeatCell[] {
	const days = eachDayOfMonth(month);
	const peopleCount = uniquePeople(events).length;
	const byDate = new Map<string, Set<string>>();
	for (const event of events) {
		if (event.canceledAt) continue;
		if (event.kind !== 'clock_in') continue;
		const set = byDate.get(event.localDate) ?? new Set();
		set.add(event.email);
		byDate.set(event.localDate, set);
	}
	return days.map((date) => {
		const presentCount = byDate.get(date)?.size ?? 0;
		const ratio = peopleCount ? presentCount / peopleCount : 0;
		let level: DayHeatCell['level'] = 0;
		if (ratio > 0.75) level = 3;
		else if (ratio > 0.25) level = 2;
		else if (ratio > 0) level = 1;
		return { date, presentCount, totalPeople: peopleCount, level };
	});
}

export type PersonalSummaryStats = {
	workedDays: number;
	weekdayCount: number;
	totalMinutes: number;
	averageClockInTime: string;
	topLocationLabel: string;
	topLocationPercent: number;
};

export function computePersonalStats(month: string, events: AttendanceEvent[]): PersonalSummaryStats {
	const byDay = groupEventsByDay(events);
	let workedDays = 0;
	let totalMinutes = 0;
	const clockInMinutes: number[] = [];
	const locationCount = new Map<string, number>();
	for (const [date, dayEvents] of byDay) {
		const day = computeDayEvents(date, dayEvents);
		if (day.clockIn) {
			workedDays += 1;
			totalMinutes += day.workedMinutes;
			clockInMinutes.push(clockInLocalMinutes(day.clockIn));
			const label = day.clockIn.locationName ?? 'Unknown';
			locationCount.set(label, (locationCount.get(label) ?? 0) + 1);
		}
	}
	const weekdayCount = eachDayOfMonth(month).filter(isWeekday).length;
	const averageClockInTime = clockInMinutes.length
		? formatTimeOfDay(clockInMinutes.reduce((a, b) => a + b, 0) / clockInMinutes.length)
		: '--:--';
	let topLocationLabel = '';
	let topLocationCount = 0;
	for (const [label, count] of locationCount) {
		if (count > topLocationCount) {
			topLocationLabel = label;
			topLocationCount = count;
		}
	}
	const topLocationPercent = workedDays ? Math.round((topLocationCount / workedDays) * 100) : 0;
	return { workedDays, weekdayCount, totalMinutes, averageClockInTime, topLocationLabel, topLocationPercent };
}

export function statusForDay(date: string, events: AttendanceEvent[]): PersonStatus {
	const day = computeDayEvents(date, events.filter((event) => event.localDate === date));
	if (day.inProgress) return 'working';
	if (day.clockIn && day.clockOut) return 'finished';
	return 'absent';
}

export type RangeMinutes = {
	minutes: number;
	workedDays: number;
	inProgress: boolean;
};

export function todayMinutes(events: AttendanceEvent[], todayDate: string): RangeMinutes {
	return rangeMinutes(events, todayDate, todayDate);
}

export function thisWeekMinutes(events: AttendanceEvent[], todayDate: string): RangeMinutes {
	return rangeMinutes(events, isoWeekStart(todayDate), todayDate);
}

export function thisMonthMinutes(events: AttendanceEvent[], todayDate: string): RangeMinutes {
	return rangeMinutes(events, `${todayDate.slice(0, 7)}-01`, todayDate);
}

export type TeamRangeAggregate = {
	minutes: number;
	workedPeople: number;
	workedDays: number;
	workingNow: number;
};

export function teamTodayAggregate(events: AttendanceEvent[], todayDate: string): TeamRangeAggregate {
	return teamRangeAggregate(events, todayDate, todayDate);
}

export function teamThisWeekAggregate(events: AttendanceEvent[], todayDate: string): TeamRangeAggregate {
	return teamRangeAggregate(events, isoWeekStart(todayDate), todayDate);
}

export function teamThisMonthAggregate(events: AttendanceEvent[], todayDate: string): TeamRangeAggregate {
	return teamRangeAggregate(events, `${todayDate.slice(0, 7)}-01`, todayDate);
}

function clockInLocalMinutes(event: AttendanceEvent): number {
	const [hh, mm] = event.localTime.split(':').map(Number);
	return (hh || 0) * 60 + (mm || 0);
}

function rangeMinutes(events: AttendanceEvent[], start: string, end: string): RangeMinutes {
	const inRange = events.filter((event) => {
		if (event.canceledAt) return false;
		return event.localDate >= start && event.localDate <= end;
	});
	const byDay = groupEventsByDay(inRange);
	let minutes = 0;
	let workedDays = 0;
	let inProgress = false;
	for (const [date, dayEvents] of byDay) {
		const day = computeDayEvents(date, dayEvents);
		if (day.clockIn) workedDays += 1;
		if (day.inProgress) inProgress = true;
		minutes += day.workedMinutes;
	}
	return { minutes, workedDays, inProgress };
}

function teamRangeAggregate(events: AttendanceEvent[], start: string, end: string): TeamRangeAggregate {
	const inRange = events.filter((event) => {
		if (event.canceledAt) return false;
		return event.localDate >= start && event.localDate <= end;
	});
	const byEmail = new Map<string, AttendanceEvent[]>();
	for (const event of inRange) {
		const list = byEmail.get(event.email) ?? [];
		list.push(event);
		byEmail.set(event.email, list);
	}
	let minutes = 0;
	let workedDays = 0;
	let workingNow = 0;
	const workedPeople = new Set<string>();
	for (const [email, personEvents] of byEmail) {
		const byDay = groupEventsByDay(personEvents);
		for (const [date, dayEvents] of byDay) {
			const day = computeDayEvents(date, dayEvents);
			if (day.clockIn) {
				workedDays += 1;
				workedPeople.add(email);
			}
			if (day.inProgress) workingNow += 1;
			minutes += day.workedMinutes;
		}
	}
	return { minutes, workedPeople: workedPeople.size, workedDays, workingNow };
}
