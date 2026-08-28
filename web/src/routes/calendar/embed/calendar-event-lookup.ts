import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';

export function findCalendarEventByID(
	appEvents: DayTaskEvent[],
	fallbackEvents: DayTaskEvent[],
	eventID: string
): DayTaskEvent | undefined {
	return appEvents.find((event) => event.id === eventID) ?? fallbackEvents.find((event) => event.id === eventID);
}
