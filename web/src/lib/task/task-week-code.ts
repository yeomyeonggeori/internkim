import type { TaskWeek } from '../../routes/task/task-types';

const millisecondsPerDay = 24 * 60 * 60 * 1000;
const isoWeekCodePattern = /^(\d{2})W(\d{1,2})$/;
const dateISOPattern = /^(\d{4})-(\d{2})-(\d{2})$/;
const lastPossibleISOWeek = 53;

export function taskWeekDateISO(date: Date): string {
	return date.toISOString().slice(0, 10);
}

export function taskWeekDateFromISO(dateISO: string): Date | null {
	const matched = dateISOPattern.exec(dateISO.trim());
	if (!matched) return null;
	const year = Number(matched[1]);
	const month = Number(matched[2]);
	const day = Number(matched[3]);
	const date = new Date(Date.UTC(year, month - 1, day));
	if (date.getUTCFullYear() !== year) return null;
	if (date.getUTCMonth() !== month - 1) return null;
	if (date.getUTCDate() !== day) return null;
	return date;
}

export function addTaskWeekDays(date: Date, days: number): Date {
	return new Date(date.getTime() + days * millisecondsPerDay);
}

export function taskWeekMondayOfDate(date: Date): Date {
	const midnight = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), date.getUTCDate()));
	return addTaskWeekDays(midnight, -((midnight.getUTCDay() + 6) % 7));
}

export function taskWeekCodeForMonday(monday: Date): string {
	const thursday = addTaskWeekDays(monday, 3);
	const year = thursday.getUTCFullYear();
	const weekOneMonday = isoWeekOneMonday(year);
	const week = Math.round((monday.getTime() - weekOneMonday.getTime()) / (7 * millisecondsPerDay)) + 1;
	return `${String(year).slice(2)}W${String(week).padStart(2, '0')}`;
}

export function taskWeekCodeForDateISO(dateISO: string): string {
	const date = taskWeekDateFromISO(dateISO);
	if (!date) return '';
	return taskWeekCodeForMonday(taskWeekMondayOfDate(date));
}

export function taskWeekMondayForCode(weekCode: string): Date | null {
	const trimmed = weekCode.trim();
	const matched = isoWeekCodePattern.exec(trimmed);
	if (!matched) {
		const legacyMondayDate = taskWeekDateFromISO(trimmed);
		return legacyMondayDate ? taskWeekMondayOfDate(legacyMondayDate) : null;
	}
	const week = Number(matched[2]);
	if (week < 1 || week > lastPossibleISOWeek) return null;
	return addTaskWeekDays(isoWeekOneMonday(2000 + Number(matched[1])), (week - 1) * 7);
}

export function taskWeekOfDate(date: Date): TaskWeek {
	return taskWeekFromMonday(taskWeekMondayOfDate(date), true);
}

export function taskWeekForCode(weekCode: string, today: Date): TaskWeek {
	const monday = taskWeekMondayForCode(weekCode);
	if (!monday) return taskWeekOfDate(today);
	const currentCode = taskWeekCodeForMonday(taskWeekMondayOfDate(today));
	return taskWeekFromMonday(monday, taskWeekCodeForMonday(monday) === currentCode);
}

function taskWeekFromMonday(monday: Date, isCurrent: boolean): TaskWeek {
	return {
		code: taskWeekCodeForMonday(monday),
		startISO: taskWeekDateISO(monday),
		endISO: taskWeekDateISO(addTaskWeekDays(monday, 6)),
		previous: taskWeekCodeForMonday(addTaskWeekDays(monday, -7)),
		next: taskWeekCodeForMonday(addTaskWeekDays(monday, 7)),
		isCurrent
	};
}

function isoWeekOneMonday(year: number): Date {
	return taskWeekMondayOfDate(new Date(Date.UTC(year, 0, 4)));
}
