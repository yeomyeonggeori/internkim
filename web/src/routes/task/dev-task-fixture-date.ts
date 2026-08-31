import type { TaskWeek } from './task-types';
import {
	addTaskWeekDays,
	taskWeekCodeForMonday,
	taskWeekDateISO,
	taskWeekMondayForCode
} from '../../lib/task/task-week-code';

const fixtureBaselineMonday = new Date(Date.UTC(2026, 5, 1));

export function buildTaskWeek(inputWeekCode: string | null | undefined): TaskWeek {
	const start = (inputWeekCode ? taskWeekMondayForCode(inputWeekCode) : null) ?? fixtureBaselineMonday;
	const code = taskWeekCodeForMonday(start);

	return {
		code,
		startISO: taskWeekDateISO(start),
		endISO: taskWeekDateISO(addTaskWeekDays(start, 6)),
		previous: taskWeekCodeForMonday(addTaskWeekDays(start, -7)),
		next: taskWeekCodeForMonday(addTaskWeekDays(start, 7)),
		isCurrent: code === taskWeekCodeForMonday(fixtureBaselineMonday)
	};
}

export function weekOffsetFromBaseline(week: TaskWeek, baselineWeekStartISO: string): number {
	return Math.trunc(dateOffset(baselineWeekStartISO, week.startISO) / 7);
}

export function monthStartISO(value: string): string {
	const date = new Date(`${value}T00:00:00Z`);
	return dateISO(new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1)));
}

export function daysInMonth(monthStart: string): number {
	const date = new Date(`${monthStart}T00:00:00Z`);
	return new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + 1, 0)).getUTCDate();
}

export function addDays(isoDate: string, days: number): string {
	return dateISO(addDate(new Date(`${isoDate}T00:00:00.000Z`), days));
}

export function dateOffset(startISO: string, targetISO: string): number {
	if (!targetISO) return -1;
	const start = new Date(`${startISO}T00:00:00.000Z`).getTime();
	const target = new Date(`${targetISO}T00:00:00.000Z`).getTime();
	return Math.floor((target - start) / (24 * 60 * 60 * 1000));
}

function addDate(date: Date, days: number): Date {
	const nextDate = new Date(date);
	nextDate.setUTCDate(nextDate.getUTCDate() + days);
	return nextDate;
}

function dateISO(date: Date): string {
	return date.toISOString().slice(0, 10);
}
