import type { Event as DayFlowEvent } from '@dayflow/core';

export function findCalendarEventByID(
	appEvents: DayFlowEvent[],
	fallbackEvents: DayFlowEvent[],
	eventID: string
): DayFlowEvent | undefined {
	return appEvents.find((event) => event.id === eventID) ?? fallbackEvents.find((event) => event.id === eventID);
}
