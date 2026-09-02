import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import { calendarColors } from './calendar-config';
import { calendarEventPayloadFromDayTaskEvent } from './calendar-event-mapping';
import {
	calendarEventPath,
	cancelCalendarEventDeleteIntent,
	createCalendarEventDeleteIntent,
	deletePersistedCalendarEvent,
	writeCalendarEvent,
	type CalendarDeleteIntent,
	type CalendarEvent
} from './calendar-event-persistence';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';

type CalendarPersistedEventActionsContext = {
	getCalendarEvents: () => DayTaskEvent[];
	updateCalendarEvent: (eventID: string, changes: Partial<DayTaskEvent>, shouldRender: boolean) => Promise<void>;
	setVisibleEvents: (events: DayTaskEvent[]) => void;
	text: {
		deleteError: string;
		saveError: string;
	};
};

export type CalendarPersistedEventActions = {
	writeEvent: (
		path: string,
		method: 'POST' | 'PUT',
		event: DayTaskEvent,
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
	const persistedEventIDs = new Map<string, string>();

	function persistedEventID(eventID: string): string {
		return persistedEventIDs.get(eventID) ?? eventID;
	}

	async function writeEvent(
		path: string,
		method: 'POST' | 'PUT',
		event: DayTaskEvent,
		expectedUpdatedAt?: string,
		mutationClientID?: string,
		mutationSequence?: number
	): Promise<CalendarEvent> {
		const timeZone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
		const currentExpectedUpdatedAt = expectedUpdatedAt
			?? (typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined);
		const eventID = method === 'PUT' ? persistedEventID(event.id) : event.id;
		const payload = {
			...calendarEventPayloadFromDayTaskEvent(event, calendarColors.lineColor, timeZone),
			eventID,
			...(method === 'PUT' && currentExpectedUpdatedAt !== undefined
				? { expectedUpdatedAt: currentExpectedUpdatedAt }
				: {}),
			...(method === 'PUT' && mutationClientID !== undefined && mutationSequence !== undefined
				? { mutationClientID, mutationSequence }
				: {})
		};
		const requestPath = eventID === event.id ? path : calendarEventPath(eventID);
		const saved = await writeCalendarEvent(requestPath, method, payload, context.text.saveError);
		if (method === 'POST' && saved.id && saved.id !== event.id) persistedEventIDs.set(event.id, saved.id);
		return saved;
	}

	async function deleteEvent(eventID: string, expectedUpdatedAt?: string): Promise<void> {
		await deletePersistedCalendarEvent(persistedEventID(eventID), expectedUpdatedAt, context.text.deleteError);
	}

	async function createDeleteIntent(
		eventID: string,
		operationID: string,
		clientID: string,
		sequence: number,
		expectedUpdatedAt: string
	): Promise<CalendarDeleteIntent> {
		return createCalendarEventDeleteIntent(
			persistedEventID(eventID),
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
		await cancelCalendarEventDeleteIntent(
			persistedEventID(eventID),
			operationID,
			clientID,
			sequence,
			context.text.deleteError
		);
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
