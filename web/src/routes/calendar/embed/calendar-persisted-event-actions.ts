import type { Event as DayFlowEvent } from '@dayflow/core';
import { calendarColors } from './calendar-config';
import { calendarEventPayloadFromDayFlowEvent } from './calendar-event-mapping';
import {
	deletePersistedCalendarEvent,
	writeCalendarEvent,
	type CalendarEvent
} from './calendar-event-persistence';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';

type CalendarPersistedEventActionsContext = {
	getCalendarEvents: () => DayFlowEvent[];
	updateCalendarEvent: (eventID: string, changes: Partial<DayFlowEvent>, shouldRender: boolean) => Promise<void>;
	setVisibleEvents: (events: DayFlowEvent[]) => void;
	text: {
		deleteError: string;
		saveError: string;
	};
};

export type CalendarPersistedEventActions = {
	writeEvent: (path: string, method: 'POST' | 'PUT', event: DayFlowEvent) => Promise<CalendarEvent>;
	deleteEvent: (eventID: string) => Promise<void>;
	applyServerMetadata: (eventID: string, event: CalendarEvent) => Promise<void>;
};

export function createCalendarPersistedEventActions(
	context: CalendarPersistedEventActionsContext,
	programmaticUpdates: CalendarProgrammaticUpdateState
): CalendarPersistedEventActions {
	async function writeEvent(path: string, method: 'POST' | 'PUT', event: DayFlowEvent): Promise<CalendarEvent> {
		const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
		const payload = calendarEventPayloadFromDayFlowEvent(event, calendarColors.lineColor, timeZone);
		return writeCalendarEvent(path, method, payload, context.text.saveError);
	}

	async function deleteEvent(eventID: string): Promise<void> {
		await deletePersistedCalendarEvent(eventID, context.text.deleteError);
	}

	async function applyServerMetadata(eventID: string, event: CalendarEvent): Promise<void> {
		const currentEvent = context.getCalendarEvents().find((candidate) => candidate.id === eventID);
		await programmaticUpdates.run(eventID, () =>
			context.updateCalendarEvent(
				eventID,
				{
					meta: {
						...(currentEvent?.meta ?? {}),
						uid: event.uid,
						location: event.location,
						color: event.color,
						participants: event.participants,
						timeZone: event.timeZone,
						createdByEmail: event.createdByEmail,
						createdByName: event.createdByName,
						createdByImage: event.createdByImage ?? '',
						updatedByEmail: event.updatedByEmail ?? '',
						updatedByName: event.updatedByName ?? '',
						updatedByImage: event.updatedByImage ?? '',
						updatedByAt: event.updatedByAt ?? '',
						updatedAt: event.updatedAt
					}
				},
				false
			)
		);
		context.setVisibleEvents(context.getCalendarEvents());
	}

	return {
		writeEvent,
		deleteEvent,
		applyServerMetadata
	};
}
