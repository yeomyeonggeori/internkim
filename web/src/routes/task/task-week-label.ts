import {
	addTaskWeekDays,
	taskWeekCodeForMonday,
	taskWeekDateFromISO,
	taskWeekDateISO,
	taskWeekMondayForCode
} from '$lib/task/task-week-code';

export type TaskWeekDateRange = {
	startISO: string;
	endISO: string;
};

export function formatTaskWeekDateRange(week: TaskWeekDateRange | null | undefined): string {
	if (!week?.startISO || !week.endISO) return '...';
	return `${formatMonthDay(week.startISO)} - ${formatMonthDay(week.endISO)}`;
}

export function formatTaskWeekCodeRange(weekCode: string): string {
	const monday = taskWeekMondayForCode(weekCode);
	if (!monday) return weekCode;
	return formatTaskWeekDateRange({
		startISO: taskWeekDateISO(monday),
		endISO: taskWeekDateISO(addTaskWeekDays(monday, 6))
	});
}

export type TaskWeekOption = {
	value: string;
	label: string;
	offsetFromCurrent: number;
};

export function taskWeekOptions(
	currentWeekStartISO: string,
	weeksBefore: number,
	weeksAfter: number
): TaskWeekOption[] {
	const currentMonday = taskWeekDateFromISO(currentWeekStartISO);
	if (!currentMonday) return [];
	const options: TaskWeekOption[] = [];
	for (let offset = weeksBefore; offset >= -weeksAfter; offset -= 1) {
		const monday = addTaskWeekDays(currentMonday, -offset * 7);
		const sunday = addTaskWeekDays(monday, 6);
		options.push({
			value: taskWeekCodeForMonday(monday),
			label: formatTaskWeekDateRange({ startISO: taskWeekDateISO(monday), endISO: taskWeekDateISO(sunday) }),
			offsetFromCurrent: offset === 0 ? 0 : -offset
		});
	}
	return options;
}

function formatMonthDay(dateISO: string): string {
	const month = Number(dateISO.slice(5, 7));
	const day = Number(dateISO.slice(8, 10));
	if (!month || !day) return dateISO;
	return `${month}/${day}`;
}
