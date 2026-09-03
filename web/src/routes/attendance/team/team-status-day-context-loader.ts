import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
import { fetchCalendarEvents } from '../../calendar/embed/calendar-event-persistence';
import { fetchTaskState } from '../../task/task-api';
import type { TaskState } from '../../task/task-types';
import { addDays } from '../shared/attendance-date';

export type TeamStatusDayContextData = {
	calendarEvents: CalendarEvent[];
	taskState: TaskState | null;
	hasCalendarEventsLoadFailed: boolean;
	hasCompletedWorkLoadFailed: boolean;
};

export async function loadTeamStatusDayContextData(
	firstDate: string,
	lastDate: string,
	fallbackMessage: string
): Promise<TeamStatusDayContextData> {
	const startDate = dateFromKey(firstDate);
	const endDate = dateFromKey(addDays(lastDate, 1));
	const [calendarResult, taskResult] = await Promise.allSettled([
		fetchCalendarEvents(startDate, endDate),
		fetchTaskState()
	]);
	return {
		calendarEvents: calendarResult.status === 'fulfilled' ? calendarResult.value : [],
		taskState: taskResult.status === 'fulfilled' ? taskResult.value : null,
		hasCalendarEventsLoadFailed: calendarResult.status === 'rejected',
		hasCompletedWorkLoadFailed: taskResult.status === 'rejected'
	};
}

function dateFromKey(date: string): Date {
	return new Date(`${date}T00:00:00`);
}
