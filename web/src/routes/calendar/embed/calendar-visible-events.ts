import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import type { CalendarDraftEventState } from './calendar-draft-events';

type DraftPopoverEventReference = {
	eventID: string;
} | null;

export function visibleEventsWithPreservedLocalEvents(
	visibleEvents: DayTaskEvent[],
	localEvents: DayTaskEvent[],
	shouldPreserveLocalEvent: (event: DayTaskEvent) => boolean
): DayTaskEvent[] {
	const visibleEventIDs = new Set(visibleEvents.map((event) => event.id));
	const preservedLocalEvents = localEvents
		.filter(shouldPreserveLocalEvent)
		.filter((event) => !visibleEventIDs.has(event.id));
	return [...visibleEvents, ...uniqueEventsByID(preservedLocalEvents)];
}

export function calendarStageEventsWithDraftPopover(
	calendarEvents: DayTaskEvent[],
	createdDraftEvents: DayTaskEvent[],
	popover: DraftPopoverEventReference
): DayTaskEvent[] {
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
	event: DayTaskEvent
): boolean {
	return draftEvents.isDraftEvent(event.id) || draftEvents.hasPendingCreate(event.id);
}

function uniqueEventsByID(events: DayTaskEvent[]): DayTaskEvent[] {
	const eventsByID = new Map(events.map((event) => [event.id, event]));
	return Array.from(eventsByID.values());
}
