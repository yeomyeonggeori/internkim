import type { AttendanceEvent } from '../attendance-context.svelte';
import { addDays, isoWeekStart } from './attendance-date';
import { computeDayEvents } from './attendance-day-events';

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
	const activeEvents = events.filter((event) => !event.canceledAt);
	let minutes = 0;
	let workedDays = 0;
	let inProgress = false;
	for (const date of eachDateInRange(start, end)) {
		const day = computeDayEvents(date, activeEvents);
		if (day.segments.length > 0) workedDays += 1;
		if (day.inProgress) inProgress = true;
		minutes += day.workedMinutes;
	}
	return { minutes, workedDays, inProgress };
}

function teamRangeAggregate(events: AttendanceEvent[], start: string, end: string): TeamRangeAggregate {
	const byEmail = new Map<string, AttendanceEvent[]>();
	for (const event of events) {
		if (event.canceledAt) continue;
		const list = byEmail.get(event.email) ?? [];
		list.push(event);
		byEmail.set(event.email, list);
	}
	let minutes = 0;
	let workedDays = 0;
	let workingNow = 0;
	const workedPeople = new Set<string>();
	for (const [email, personEvents] of byEmail) {
		for (const date of eachDateInRange(start, end)) {
			const day = computeDayEvents(date, personEvents);
			if (day.segments.length > 0) {
				workedDays += 1;
				workedPeople.add(email);
			}
			if (day.inProgress) workingNow += 1;
			minutes += day.workedMinutes;
		}
	}
	return { minutes, workedPeople: workedPeople.size, workedDays, workingNow };
}

function eachDateInRange(start: string, end: string): string[] {
	const dates: string[] = [];
	for (let date = start; date <= end; date = addDays(date, 1)) {
		dates.push(date);
	}
	return dates;
}
