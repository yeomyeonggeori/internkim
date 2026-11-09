import type { CalendarHolidayLocale } from '$lib/calendar/holiday';
import { supabaseCalendarHolidays } from '$lib/calendar/supabase-calendar-holidays';
import { taskWeekDateFromISO } from '$lib/task/task-week-code';

export type TaskRoadmapHolidays = {
	dates: ReadonlySet<string>;
	titles: ReadonlyMap<string, string>;
};

export const noTaskRoadmapHolidays: TaskRoadmapHolidays = { dates: new Set(), titles: new Map() };

export async function loadTaskRoadmapHolidays(firstDayISO: string, lastDayISO: string, locale: CalendarHolidayLocale): Promise<TaskRoadmapHolidays> {
	const first = taskWeekDateFromISO(firstDayISO);
	const last = taskWeekDateFromISO(lastDayISO);
	if (!first || !last) return noTaskRoadmapHolidays;
	const answer = await supabaseCalendarHolidays(first, last, locale);
	return {
		dates: new Set(answer.holidays.map((holiday) => holiday.date)),
		titles: new Map(answer.holidays.map((holiday) => [holiday.date, holiday.title]))
	};
}
