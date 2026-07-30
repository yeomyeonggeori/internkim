import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import type { CalendarDraftEventState } from './calendar-draft-events';

type DraftPopoverEventReference = {
	eventID: string;
} | null;

export function visibleEventsWithPreservedLocalEvents(
	visibleEvents: DayFlowEvent[],
	localEvents: DayFlowEvent[],
	shouldPreserveLocalEvent: (event: DayFlowEvent) => boolean
): DayFlowEvent[] {
	const visibleEventIDs = new Set(visibleEvents.map((event) => event.id));
	const preservedLocalEvents = localEvents
		.filter(shouldPreserveLocalEvent)
		.filter((event) => !visibleEventIDs.has(event.id));
	return [...visibleEvents, ...uniqueEventsByID(preservedLocalEvents)];
}

export function calendarStageEventsWithDraftPopover(
	calendarEvents: DayFlowEvent[],
	createdDraftEvents: DayFlowEvent[],
	popover: DraftPopoverEventReference
): DayFlowEvent[] {
	const popoverDraftEvent = popover
		? createdDraftEvents.find((event) => event.id === popover.eventID)
		: undefined;
	const otherDraftEvents = popoverDraftEvent
		? createdDraftEvents.filter((event) => event.id !== popoverDraftEvent.id)
		: createdDraftEvents;
	return [...calendarEvents, ...otherDraftEvents, ...(popoverDraftEvent ? [popoverDraftEvent] : [])];
}

export function shouldPreserveLocalCalendarEvent(
	draftEvents: CalendarDraftEventState,
	event: DayFlowEvent
): boolean {
	return draftEvents.isDraftEvent(event.id) || draftEvents.hasPendingCreate(event.id);
}

function uniqueEventsByID(events: DayFlowEvent[]): DayFlowEvent[] {
	const eventsByID = new Map(events.map((event) => [event.id, event]));
	return Array.from(eventsByID.values());
}
