import type { FlowWeek } from './flow-types';

export function buildFlowWeek(inputWeekCode: string | null | undefined): FlowWeek {
	const fallbackStart = mondayForWeekCode('26W23');
	const start = inputWeekCode ? mondayForWeekCode(inputWeekCode) : fallbackStart;
	const code = weekCodeForMonday(start);

	return {
		code,
		startISO: dateISO(start),
		endISO: dateISO(addDate(start, 6)),
		previous: weekCodeForMonday(addDate(start, -7)),
		next: weekCodeForMonday(addDate(start, 7)),
		isCurrent: code === weekCodeForMonday(fallbackStart)
	};
}

export function weekOffsetFromBaseline(week: FlowWeek, baselineWeekStartISO: string): number {
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

function mondayForWeekCode(weekCode: string): Date {
	const matched = /^(\d{2})W(\d{1,2})$/.exec(weekCode.trim());
	if (!matched) return mondayForWeekCode('26W23');

	const year = 2000 + Number(matched[1]);
	const week = Number(matched[2]);
	const januaryFourth = new Date(Date.UTC(year, 0, 4));
	const isoWeekOneMonday = addDate(januaryFourth, -((januaryFourth.getUTCDay() + 6) % 7));
	return addDate(isoWeekOneMonday, (week - 1) * 7);
}

function weekCodeForMonday(monday: Date): string {
	const thursday = addDate(monday, 3);
	const year = thursday.getUTCFullYear();
	const januaryFourth = new Date(Date.UTC(year, 0, 4));
	const isoWeekOneMonday = addDate(januaryFourth, -((januaryFourth.getUTCDay() + 6) % 7));
	const week = Math.floor((monday.getTime() - isoWeekOneMonday.getTime()) / (7 * 24 * 60 * 60 * 1000)) + 1;
	return `${String(year).slice(2)}W${String(week).padStart(2, '0')}`;
}

function addDate(date: Date, days: number): Date {
	const nextDate = new Date(date);
	nextDate.setUTCDate(nextDate.getUTCDate() + days);
	return nextDate;
}

function dateISO(date: Date): string {
	return date.toISOString().slice(0, 10);
}
