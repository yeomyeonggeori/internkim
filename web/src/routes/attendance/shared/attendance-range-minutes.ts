import type { AttendanceEvent } from '../attendance-context.svelte';
import { isoWeekStart } from './attendance-date';
import { computeDayEvents, groupEventsByDay } from './attendance-day-events';

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
