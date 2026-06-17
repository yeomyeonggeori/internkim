import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEventLoader } from './calendar-event-loader';

type CalendarPageVisibilityContext = {
	getVisibleEvents: () => DayFlowEvent[];
	renderVisibleEvents: CalendarEventLoader['renderVisibleEvents'];
	setWorkCalendarVisible: (isVisible: boolean) => void;
};

export type CalendarPageVisibilityActions = {
	setWorkCalendarVisibility: (isVisible: boolean) => void;
};

export function createCalendarPageVisibility(
	context: CalendarPageVisibilityContext
): CalendarPageVisibilityActions {
	function setWorkCalendarVisibility(isVisible: boolean): void {
		context.setWorkCalendarVisible(isVisible);
		context.renderVisibleEvents(context.getVisibleEvents());
	}

	return { setWorkCalendarVisibility };
}
