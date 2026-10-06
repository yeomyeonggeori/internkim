import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import { calendarColors } from './calendar-config';
import { calendarEventPayloadFromDayTaskEvent } from './calendar-event-mapping';
import {
	deletePersistedCalendarEvent,
	writeCalendarEvent,
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
		isNewEvent: boolean,
		event: DayTaskEvent,
		expectedUpdatedAt?: string,
		mutationClientID?: string,
		mutationSequence?: number
	) => Promise<CalendarEvent>;
	deleteEvent: (eventID: string, expectedUpdatedAt?: string) => Promise<void>;
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
		isNewEvent: boolean,
		event: DayTaskEvent,
		expectedUpdatedAt?: string,
		mutationClientID?: string,
		mutationSequence?: number
	): Promise<CalendarEvent> {
		const currentExpectedUpdatedAt = expectedUpdatedAt
			?? (typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined);
		const eventID = isNewEvent ? event.id : persistedEventID(event.id);
		const payload = {
			...calendarEventPayloadFromDayTaskEvent(event, calendarColors.lineColor),
			eventID,
			...(!isNewEvent && currentExpectedUpdatedAt !== undefined
				? { expectedUpdatedAt: currentExpectedUpdatedAt }
				: {}),
			...(!isNewEvent && mutationClientID !== undefined && mutationSequence !== undefined
				? { mutationClientID, mutationSequence }
				: {})
		};
		const saved = await writeCalendarEvent(isNewEvent, payload);
		if (isNewEvent && saved.id && saved.id !== event.id) persistedEventIDs.set(event.id, saved.id);
		return saved;
	}

	async function deleteEvent(eventID: string, expectedUpdatedAt?: string): Promise<void> {
		await deletePersistedCalendarEvent(persistedEventID(eventID), expectedUpdatedAt);
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
						isOpenToCompany: event.isOpenToCompany ?? false,
						reminderMinutesBefore: event.reminderMinutesBefore ?? null,
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
