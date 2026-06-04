import { ViewType } from '@dayflow/svelte';

export function shiftedCalendarToolbarDate(date: Date, viewType: ViewType, direction: -1 | 1): Date {
	const nextDate = new Date(date);
	if (viewType === ViewType.DAY) {
		nextDate.setDate(nextDate.getDate() + direction);
		return nextDate;
	}
	if (viewType === ViewType.WEEK) {
		nextDate.setDate(nextDate.getDate() + direction * 7);
		return nextDate;
	}
	nextDate.setMonth(nextDate.getMonth() + direction);
	return nextDate;
}

export function startOfMonthWindow(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth() - 1, 1);
}

export function endOfMonthWindow(date: Date): Date {
	return new Date(date.getFullYear(), date.getMonth() + 2, 1);
}
