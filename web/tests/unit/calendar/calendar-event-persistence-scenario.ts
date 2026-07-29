import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import { mock } from 'bun:test';

import { CalendarDraftEventState } from '../../../src/routes/calendar/embed/calendar-draft-events';
import type { CalendarDraftEventDOMActions } from '../../../src/routes/calendar/embed/calendar-draft-event-dom';
import type { CalendarEventActionsContext } from '../../../src/routes/calendar/embed/calendar-event-actions';
import {
	CalendarPersistenceError,
	type CalendarEvent
} from '../../../src/routes/calendar/embed/calendar-event-persistence';
import type { CalendarPersistedEventActions } from '../../../src/routes/calendar/embed/calendar-persisted-event-actions';
import { CalendarProgrammaticUpdateState } from '../../../src/routes/calendar/embed/calendar-programmatic-updates';

export type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';

export const toastErrorMessages: string[] = [];
export const targetUnavailableMessage = 'Reconnect the account or select a writable calendar.';

mock.module('svelte-sonner', () => ({
	toast: Object.assign(() => {}, {
		dismiss: () => {},
		error: (message: string) => toastErrorMessages.push(message)
	})
}));

const { createCalendarEventPersistenceActions } = await import(
	'../../../src/routes/calendar/embed/calendar-event-persistence-actions'
);
const { createCalendarEventActions } = await import(
	'../../../src/routes/calendar/embed/calendar-event-actions'
);

export function createPersistenceScenario(
	initialEvents: DayFlowEvent[],
	refreshedEvents: () => DayFlowEvent[] = () => initialEvents,
	persistedOverrides: Partial<CalendarPersistedEventActions> = {},
	useDefaultNotifier = false,
	waitForDeleteIntentCancellationRetry: (delay: number) => Promise<void> = async () => {}
) {
	let calendarEvents = [...initialEvents];
	let calendarRefreshCount = 0;
	let pendingLoadInvalidationCount = 0;
	let undoPendingDelete: () => void = () => {};
	const notifications: string[] = [];
	const restoredEventTitles: string[] = [];
	const savingStates: boolean[] = [];
	const draftEvents = new CalendarDraftEventState();
	const context: CalendarEventActionsContext = {
		isBrowser: () => false,
		getCurrentDate: () => new Date('2026-07-16T00:00:00Z'),
		getStageElement: () => null,
		getSelectedAuditEventID: () => null,
		setSelectedAuditEventID: () => {},
		getCalendarEvents: () => calendarEvents,
		addCalendarEvent: (event) => {
			calendarEvents = [...calendarEvents, event];
		},
		restoreCalendarEvent: (event) => {
			restoredEventTitles.push(event.title);
			calendarEvents = [...calendarEvents.filter((candidate) => candidate.id !== event.id), event];
		},
		removeCalendarEvent: (eventID) => {
			calendarEvents = calendarEvents.filter((event) => event.id !== eventID);
		},
		updateCalendarEvent: async () => {},
		setEventCount: () => {},
		setVisibleEvents: (events) => {
			calendarEvents = [...events];
		},
		setIsSaving: (isSaving) => {
			savingStates.push(isSaving);
		},
		setStatusMessage: () => {},
		setErrorMessage: () => {},
		openEventDetails: () => {},
		openMobileEventEditor: () => {},
		notifyEventsChanged: () => {},
		refreshCalendar: async () => {
			calendarRefreshCount += 1;
			calendarEvents = [...refreshedEvents()];
		},
		invalidatePendingEventLoad: () => {
			pendingLoadInvalidationCount += 1;
		},
		text: {
			calendarDeleteVersionConflictError:
				'This event changed elsewhere, so it could not be deleted. The latest server version has been reloaded.',
			calendarEventVersionConflictError:
				'This event changed elsewhere. The latest server version has been reloaded.',
			calendarTargetUnavailableError: targetUnavailableMessage,
			deleteError: 'Could not delete the event.',
			deleteUndoAction: 'Undo',
			deleteUndoMessage: 'Event deleted.',
			draftTitlePlaceholder: 'New Event',
			saveError: 'Could not save the event.',
			shared: 'Shared'
		}
	};
	const persistedEvents: CalendarPersistedEventActions = {
		writeEvent: async () => {
			throw targetUnavailableError();
		},
		deleteEvent: async () => {
			throw targetUnavailableError();
		},
		createDeleteIntent: async () => {
			throw targetUnavailableError();
		},
		cancelDeleteIntent: async () => {
			throw targetUnavailableError();
		},
		applyServerMetadata: async () => {},
		...persistedOverrides
	};
	const draftEventDOM: CalendarDraftEventDOMActions = {
		openEventDetailsAfterRender: () => {},
		scheduleDraftTitleInputPlaceholderUpdates: () => {},
		scheduleDraftEventVisibilitySync: () => {}
	};
	const actions = createCalendarEventPersistenceActions(
		{
			context,
			draftEvents,
			draftEventDOM,
			programmaticUpdates: new CalendarProgrammaticUpdateState(),
			refreshEventCountAfterRender: () => {},
			refreshLocalEventSnapshot: () => {},
			resetDraftEventTitle: async () => {}
		},
		{
			createPersistedEventActions: () => persistedEvents,
			dismissDeleteUndoToast: () => {},
			waitForDeleteIntentCancellationRetry,
			showDeleteUndoToast: (options) => {
				undoPendingDelete = options.undo;
			},
			...(useDefaultNotifier ? {} : { notifyError: (message: string) => notifications.push(message) })
		}
	);
	return {
		actions,
		draftEvents,
		events: () => calendarEvents,
		notifications,
		restoredEventTitles,
		refreshCount: () => calendarRefreshCount,
		pendingLoadInvalidationCount: () => pendingLoadInvalidationCount,
		undoPendingDelete: () => undoPendingDelete(),
		context,
		savingStates
	};
}

export function createEventActionsForScenario(
	scenario: ReturnType<typeof createPersistenceScenario>
) {
	return createCalendarEventActions(
		scenario.context,
		scenario.draftEvents,
		new CalendarProgrammaticUpdateState()
	);
}

export function calendarTestEvent(eventID: string, title: string): DayFlowEvent {
	return createEvent({
		id: eventID,
		title,
		start: new Date('2026-07-16T01:00:00Z'),
		end: new Date('2026-07-16T02:00:00Z'),
		allDay: false,
		calendarId: 'internkim'
	});
}

export function calendarVersionedTestEvent(
	eventID: string,
	title: string,
	updatedAt: string
): DayFlowEvent {
	return createEvent({
		id: eventID,
		title,
		start: new Date('2026-07-16T01:00:00Z'),
		end: new Date('2026-07-16T02:00:00Z'),
		allDay: false,
		calendarId: 'internkim',
		meta: { updatedAt }
	});
}

export function targetUnavailableError(): CalendarPersistenceError {
	return new CalendarPersistenceError('calendar_target_unavailable', 'Could not persist the event.');
}

export function calendarServerEvent(eventID: string, title: string): CalendarEvent {
	return {
		id: eventID,
		uid: `${eventID}@internkim`,
		title,
		description: '',
		location: '',
		startISO: '2026-07-16T01:00:00Z',
		endISO: '2026-07-16T02:00:00Z',
		timeZone: 'UTC',
		isAllDay: false,
		color: '#2563eb',
		createdByEmail: 'admin@example.com',
		createdByName: 'Admin',
		updatedAt: '2026-07-16T00:00:00Z'
	};
}
