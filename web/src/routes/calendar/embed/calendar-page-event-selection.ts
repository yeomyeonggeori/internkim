import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import { clearFocusedCalendarEventElements, focusCalendarEventElement } from './calendar-event-elements';
import type { CalendarEventStore } from './calendar-event-store.svelte';

type CalendarPageEventSelectionContext = {
	eventStore: CalendarEventStore;
	getPendingEventID: () => string;
	getStageElement: () => HTMLElement | null;
	saveUpdatedEvent: (event: DayTaskEvent, previousEvent?: DayTaskEvent) => Promise<void>;
	setPendingEventID: (eventID: string) => void;
	setSelectedAuditEventID: (eventID: string | null) => void;
	setVisibleEvents: (events: DayTaskEvent[]) => void;
};

export type CalendarPageEventSelectionActions = {
	clearSelectedEvent: () => void;
	openPendingCalendarEvent: (events: DayTaskEvent[]) => void;
	replaceLocalCalendarEvent: (event: DayTaskEvent) => void;
	saveMovedMonthEvent: (event: DayTaskEvent) => Promise<void>;
	selectCalendarEvent: (eventID: string) => void;
};

export function createCalendarPageEventSelection(
	context: CalendarPageEventSelectionContext
): CalendarPageEventSelectionActions {
	function selectCalendarEvent(eventID: string): void {
		context.setSelectedAuditEventID(eventID);
		focusCalendarEventElement(context.getStageElement(), eventID);
		requestAnimationFrame(() => focusCalendarEventElement(context.getStageElement(), eventID));
	}

	function replaceLocalCalendarEvent(event: DayTaskEvent): void {
		context.eventStore.applyEventsChanges({ delete: [event.id], add: [event] });
		context.setVisibleEvents(context.eventStore.getAllEvents());
	}

	async function saveMovedMonthEvent(event: DayTaskEvent): Promise<void> {
		const previousEvent = context.eventStore.getAllEvents().find((candidate) => candidate.id === event.id);
		replaceLocalCalendarEvent(event);
		await context.saveUpdatedEvent(event, previousEvent);
	}

	function clearSelectedEvent(): void {
		context.setSelectedAuditEventID(null);
		clearFocusedCalendarEventElements(context.getStageElement());
	}

	function openPendingCalendarEvent(events: DayTaskEvent[]): void {
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
