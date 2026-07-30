import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';

export function findCalendarEventByID(
	appEvents: DayFlowEvent[],
	fallbackEvents: DayFlowEvent[],
	eventID: string
): DayFlowEvent | undefined {
	return appEvents.find((event) => event.id === eventID) ?? fallbackEvents.find((event) => event.id === eventID);
}
