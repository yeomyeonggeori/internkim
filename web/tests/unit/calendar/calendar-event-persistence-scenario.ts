import { createCalendarModelEvent as createEvent, type CalendarModelEvent as DayTaskEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { mock } from 'bun:test';

import { CalendarDraftEventState } from '../../../src/routes/calendar/embed/calendar-draft-events';
import type { CalendarDraftEventDOMActions } from '../../../src/routes/calendar/embed/calendar-draft-event-dom';
import type { CalendarEventActionsContext } from '../../../src/routes/calendar/embed/calendar-event-actions';
import type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';
import type { CalendarPersistedEventActions } from '../../../src/routes/calendar/embed/calendar-persisted-event-actions';
import { CalendarProgrammaticUpdateState } from '../../../src/routes/calendar/embed/calendar-programmatic-updates';
import { ToolRefused } from '../../../src/lib/public-api-call';

export type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';

export const toastErrorMessages: string[] = [];
export const saveFailureMessage = 'Could not save the event.';
export const deleteFailureMessage = 'Could not delete the event.';
export const saveVersionConflictMessage =
	'This event changed elsewhere. The latest server version has been reloaded.';
export const deleteVersionConflictMessage =
	'This event changed elsewhere, so it could not be deleted. The latest server version has been reloaded.';

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
	initialEvents: DayTaskEvent[],
	refreshedEvents: () => DayTaskEvent[] = () => initialEvents,
	persistedOverrides: Partial<CalendarPersistedEventActions> = {},
	useDefaultNotifier = false
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
		defaultEventParticipants: () => [],
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
			calendarDeleteVersionConflictError: deleteVersionConflictMessage,
			calendarEventVersionConflictError: saveVersionConflictMessage,
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
			throw unknownPersistenceError();
		},
		deleteEvent: async () => {
			throw unknownPersistenceError();
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

export function calendarTestEvent(eventID: string, title: string): DayTaskEvent {
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
): DayTaskEvent {
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

export function unknownPersistenceError(): Error {
	return new Error('the record refused the write');
}

export function versionConflictRefusal(): ToolRefused {
	return new ToolRefused(
		'this event changed since the version named here was read',
		'calendar_event_version_conflict',
		409
	);
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
