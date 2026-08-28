import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
import type { TaskState } from '../../task/task-types';
import { calendarEventDetailsForPersonDay } from './team-status-calendar-context';
import { completedTasksForPersonDay } from './team-status-task-context';

export type TeamStatusDayContextPerson = {
	email: string;
	displayName: string;
	mattermostUsername?: string;
};

export type TeamStatusCalendarEventDetail = {
	id: string;
	title: string;
	timeLabel: string;
	location?: string;
	calendarEvent: CalendarEvent;
};

export type TeamStatusCompletedTaskDetail = {
	id: string;
	title: string;
	ownerName: string;
	collaboratorNames: string[];
	task: TaskState['tasks'][number];
};

export type TeamStatusDayContext = {
	calendarEvents: TeamStatusCalendarEventDetail[];
	completedTasks: TeamStatusCompletedTaskDetail[];
	isCalendarEventsLoading: boolean;
	isCompletedWorkLoading: boolean;
	hasCalendarEventsLoadFailed: boolean;
	hasCompletedWorkLoadFailed: boolean;
};

export type TeamStatusDayContextLoadState = {
	isCalendarEventsLoading: boolean;
	isCompletedWorkLoading: boolean;
	hasCalendarEventsLoadFailed: boolean;
	hasCompletedWorkLoadFailed: boolean;
};

export function buildTeamStatusDayContext(
	person: TeamStatusDayContextPerson,
	date: string,
	calendarEvents: CalendarEvent[],
	taskState: TaskState | null,
	localeCode: string,
	allDayLabel: string,
	loadState: TeamStatusDayContextLoadState
): TeamStatusDayContext {
	return {
		calendarEvents: calendarEventDetailsForPersonDay(person, date, calendarEvents, localeCode, allDayLabel),
		completedTasks: completedTasksForPersonDay(person, date, taskState),
		isCalendarEventsLoading: loadState.isCalendarEventsLoading,
		isCompletedWorkLoading: loadState.isCompletedWorkLoading,
		hasCalendarEventsLoadFailed: loadState.hasCalendarEventsLoadFailed,
		hasCompletedWorkLoadFailed: loadState.hasCompletedWorkLoadFailed
	};
}
