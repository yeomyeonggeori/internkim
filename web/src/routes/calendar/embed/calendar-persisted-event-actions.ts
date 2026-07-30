import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { calendarColors } from './calendar-config';
import { calendarEventPayloadFromDayFlowEvent } from './calendar-event-mapping';
import {
	cancelCalendarEventDeleteIntent,
	createCalendarEventDeleteIntent,
	deletePersistedCalendarEvent,
	writeCalendarEvent,
	type CalendarDeleteIntent,
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
		expectedUpdatedAt?: string,
		mutationClientID?: string,
		mutationSequence?: number
	) => Promise<CalendarEvent>;
	deleteEvent: (eventID: string, expectedUpdatedAt?: string) => Promise<void>;
	createDeleteIntent: (
		eventID: string,
		operationID: string,
		clientID: string,
		sequence: number,
		expectedUpdatedAt: string
	) => Promise<CalendarDeleteIntent>;
	cancelDeleteIntent: (eventID: string, operationID: string, clientID: string, sequence: number) => Promise<void>;
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
		expectedUpdatedAt?: string,
		mutationClientID?: string,
		mutationSequence?: number
	): Promise<CalendarEvent> {
		const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
		const currentExpectedUpdatedAt = expectedUpdatedAt
			?? (typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined);
		const payload = {
			...calendarEventPayloadFromDayFlowEvent(event, calendarColors.lineColor, timeZone),
			...(method === 'PUT' && currentExpectedUpdatedAt !== undefined
				? { expectedUpdatedAt: currentExpectedUpdatedAt }
				: {}),
			...(method === 'PUT' && mutationClientID !== undefined && mutationSequence !== undefined
				? { mutationClientID, mutationSequence }
				: {})
		};
		return writeCalendarEvent(path, method, payload, context.text.saveError);
	}

	async function deleteEvent(eventID: string, expectedUpdatedAt?: string): Promise<void> {
		await deletePersistedCalendarEvent(eventID, expectedUpdatedAt, context.text.deleteError);
	}

	async function createDeleteIntent(
		eventID: string,
		operationID: string,
		clientID: string,
		sequence: number,
		expectedUpdatedAt: string
	): Promise<CalendarDeleteIntent> {
		return createCalendarEventDeleteIntent(
			eventID,
			operationID,
			clientID,
			sequence,
			expectedUpdatedAt,
			context.text.deleteError
		);
	}

	async function cancelDeleteIntent(
		eventID: string,
		operationID: string,
		clientID: string,
		sequence: number
	): Promise<void> {
		await cancelCalendarEventDeleteIntent(eventID, operationID, clientID, sequence, context.text.deleteError);
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
		createDeleteIntent,
		cancelDeleteIntent,
		applyServerMetadata
	};
}
