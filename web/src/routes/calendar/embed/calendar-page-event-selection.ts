import type { Event as DayFlowEvent } from '@dayflow/core';
import { clearFocusedCalendarEventElements, focusCalendarEventElement } from './calendar-event-elements';

type CalendarPageEventSelectionCalendar = {
	app: {
		applyEventsChanges: (changes: { delete: string[]; add: DayFlowEvent[] }) => void;
		getAllEvents: () => DayFlowEvent[];
		selectEvent: (eventID: string | null) => void;
	};
};

type CalendarPageEventSelectionContext = {
	calendar: CalendarPageEventSelectionCalendar;
	getPendingEventID: () => string;
	getStageElement: () => HTMLElement | null;
	saveUpdatedEvent: (event: DayFlowEvent, previousEvent?: DayFlowEvent) => Promise<void>;
	setPendingEventID: (eventID: string) => void;
	setSelectedAuditEventID: (eventID: string | null) => void;
	setVisibleEvents: (events: DayFlowEvent[]) => void;
};

export type CalendarPageEventSelectionActions = {
	clearSelectedEvent: () => void;
	openPendingCalendarEvent: (events: DayFlowEvent[]) => void;
	replaceLocalCalendarEvent: (event: DayFlowEvent) => void;
	saveMovedMonthEvent: (event: DayFlowEvent) => Promise<void>;
	selectCalendarEvent: (eventID: string) => void;
};

export function createCalendarPageEventSelection(
	context: CalendarPageEventSelectionContext
): CalendarPageEventSelectionActions {
	function selectCalendarEvent(eventID: string): void {
		context.setSelectedAuditEventID(eventID);
		focusCalendarEventElement(context.getStageElement(), eventID);
		context.calendar.app.selectEvent(eventID);
		requestAnimationFrame(() => focusCalendarEventElement(context.getStageElement(), eventID));
	}

	function replaceLocalCalendarEvent(event: DayFlowEvent): void {
		context.calendar.app.applyEventsChanges({ delete: [event.id], add: [event] });
		context.setVisibleEvents(context.calendar.app.getAllEvents());
	}

	async function saveMovedMonthEvent(event: DayFlowEvent): Promise<void> {
		const previousEvent = context.calendar.app.getAllEvents().find((candidate) => candidate.id === event.id);
		replaceLocalCalendarEvent(event);
		await context.saveUpdatedEvent(event, previousEvent);
	}

	function clearSelectedEvent(): void {
		context.setSelectedAuditEventID(null);
		context.calendar.app.selectEvent(null);
		clearFocusedCalendarEventElements(context.getStageElement());
	}

	function openPendingCalendarEvent(events: DayFlowEvent[]): void {
		const pendingEventID = context.getPendingEventID();
		if (!pendingEventID) return;
		if (!events.some((event) => event.id === pendingEventID)) return;
		context.setPendingEventID('');
		selectCalendarEvent(pendingEventID);
	}

	return {
		clearSelectedEvent,
		openPendingCalendarEvent,
		replaceLocalCalendarEvent,
		saveMovedMonthEvent,
		selectCalendarEvent
	};
}
