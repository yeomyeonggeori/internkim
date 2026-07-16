import type { Event as DayFlowEvent } from '@dayflow/core';
import { calendarColors } from './calendar-config';
import { calendarEventPayloadFromDayFlowEvent } from './calendar-event-mapping';
import {
	deletePersistedCalendarEventOnPageHide,
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
	writeEvent: (
		path: string,
		method: 'POST' | 'PUT',
		event: DayFlowEvent,
		expectedUpdatedAt?: string
	) => Promise<CalendarEvent>;
	deleteEvent: (eventID: string, expectedUpdatedAt?: string) => Promise<void>;
	deleteEventOnPageHide: (eventID: string, expectedUpdatedAt?: string) => void;
	applyServerMetadata: (eventID: string, event: CalendarEvent) => Promise<void>;
};

export function createCalendarPersistedEventActions(
	context: CalendarPersistedEventActionsContext,
	programmaticUpdates: CalendarProgrammaticUpdateState
): CalendarPersistedEventActions {
	async function writeEvent(
		path: string,
		method: 'POST' | 'PUT',
		event: DayFlowEvent,
		expectedUpdatedAt?: string
	): Promise<CalendarEvent> {
		const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
		const currentExpectedUpdatedAt = expectedUpdatedAt
			?? (typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined);
		const payload = {
			...calendarEventPayloadFromDayFlowEvent(event, calendarColors.lineColor, timeZone),
			...(method === 'PUT' && currentExpectedUpdatedAt !== undefined
				? { expectedUpdatedAt: currentExpectedUpdatedAt }
				: {})
		};
		return writeCalendarEvent(path, method, payload, context.text.saveError);
	}

	async function deleteEvent(eventID: string, expectedUpdatedAt?: string): Promise<void> {
		await deletePersistedCalendarEvent(eventID, expectedUpdatedAt, context.text.deleteError);
	}

	function deleteEventOnPageHide(eventID: string, expectedUpdatedAt?: string): void {
		deletePersistedCalendarEventOnPageHide(eventID, expectedUpdatedAt);
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
		deleteEventOnPageHide,
		applyServerMetadata
	};
}
