import type { CalendarEvent } from '../../calendar/embed/calendar-event-persistence';
import type { FlowState } from '../../flow/flow-types';
import { calendarEventDetailsForPersonDay } from './team-status-calendar-context';
import { completedFlowTasksForPersonDay } from './team-status-flow-context';

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
	task: FlowState['tasks'][number];
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
	flowState: FlowState | null,
	localeCode: string,
	allDayLabel: string,
	loadState: TeamStatusDayContextLoadState
): TeamStatusDayContext {
	return {
		calendarEvents: calendarEventDetailsForPersonDay(person, date, calendarEvents, localeCode, allDayLabel),
		completedTasks: completedFlowTasksForPersonDay(person, date, flowState),
		isCalendarEventsLoading: loadState.isCalendarEventsLoading,
		isCompletedWorkLoading: loadState.isCompletedWorkLoading,
		hasCalendarEventsLoadFailed: loadState.hasCalendarEventsLoadFailed,
		hasCompletedWorkLoadFailed: loadState.hasCompletedWorkLoadFailed
	};
}
