import { createEvent, type Event as DayFlowEvent } from '@dayflow/core';
import { expect, mock, test } from 'bun:test';

import { CalendarDraftEventState } from '../../../src/routes/calendar/embed/calendar-draft-events';
import type { CalendarDraftEventDOMActions } from '../../../src/routes/calendar/embed/calendar-draft-event-dom';
import type { CalendarEventActionsContext } from '../../../src/routes/calendar/embed/calendar-event-actions';
import {
	CalendarPersistenceError,
	type CalendarEvent
} from '../../../src/routes/calendar/embed/calendar-event-persistence';
import type { CalendarPersistedEventActions } from '../../../src/routes/calendar/embed/calendar-persisted-event-actions';
import { CalendarProgrammaticUpdateState } from '../../../src/routes/calendar/embed/calendar-programmatic-updates';

const toastErrorMessages: string[] = [];

mock.module('svelte-sonner', () => ({
	toast: Object.assign(() => {}, {
		dismiss: () => {},
		error: (message: string) => toastErrorMessages.push(message)
	})
}));

const { createCalendarEventPersistenceActions } = await import(
	'../../../src/routes/calendar/embed/calendar-event-persistence-actions'
);

const targetUnavailableMessage = 'Reconnect the account or select a writable calendar.';

test('keeps the latest draft after create reports target unavailable', async () => {
	const originalDraft = calendarTestEvent('draft-event', '');
	const titledDraft = calendarTestEvent('draft-event', 'Draft title');
	const scenario = createPersistenceScenario([titledDraft]);
	scenario.draftEvents.addCreatedEvent(originalDraft);

	await scenario.actions.saveUpdatedEvent(titledDraft);

	expect(scenario.draftEvents.isDraftEvent(titledDraft.id)).toBe(true);
	expect(scenario.draftEvents.createdEvents().map((event) => event.title)).toEqual(['Draft title']);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
});

test('deletes an event created on the server after the pending draft was deleted', async () => {
	const originalDraft = calendarTestEvent('pending-delete-event', '');
	const titledDraft = calendarTestEvent('pending-delete-event', 'Draft title');
	const deletedEventIDs: string[] = [];
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([titledDraft], () => [], {
		writeEvent: async () => pendingWrite,
		deleteEvent: async (eventID) => {
			deletedEventIDs.push(eventID);
		}
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const savePromise = scenario.actions.saveUpdatedEvent(titledDraft);
	await scenario.actions.deleteEvent(titledDraft.id);
	resolveWrite(calendarServerEvent(titledDraft.id, titledDraft.title));
	await savePromise;

	expect(scenario.events()).toEqual([]);
	expect(scenario.draftEvents.isDraftEvent(titledDraft.id)).toBe(false);
	expect(deletedEventIDs).toEqual([titledDraft.id]);
});

test('forgets a pending draft deleted before its failed create request completes', async () => {
	const originalDraft = calendarTestEvent('pending-failure-event', '');
	const titledDraft = calendarTestEvent('pending-failure-event', 'Draft title');
	let rejectWrite: (error: Error) => void = () => {};
	const pendingWrite = new Promise<CalendarEvent>((_, reject) => {
		rejectWrite = reject;
	});
	const scenario = createPersistenceScenario([titledDraft], () => [], {
		writeEvent: async () => pendingWrite
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const savePromise = scenario.actions.saveUpdatedEvent(titledDraft);
	await scenario.actions.deleteEvent(titledDraft.id);
	rejectWrite(targetUnavailableError());
	await savePromise;

	expect(scenario.events()).toEqual([]);
	expect(scenario.draftEvents.isDraftEvent(titledDraft.id)).toBe(false);
	expect(scenario.notifications).toEqual([]);
});

test('restores the previous event after an optimistic update fails', async () => {
	const previousEvent = calendarTestEvent('updated-event', 'Previous title');
	const updatedEvent = calendarTestEvent('updated-event', 'Updated title');
	const scenario = createPersistenceScenario([updatedEvent]);

	await scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);

	expect(scenario.events().map((event) => event.title)).toEqual(['Previous title']);
	expect(scenario.refreshCount()).toBe(0);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
});

test('uses toast error as the default persistence failure notification', async () => {
	toastErrorMessages.length = 0;
	const previousEvent = calendarTestEvent('toast-event', 'Previous title');
	const updatedEvent = calendarTestEvent('toast-event', 'Updated title');
	const scenario = createPersistenceScenario([updatedEvent], () => [previousEvent], {}, true);

	await scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);

	expect(toastErrorMessages).toEqual([targetUnavailableMessage]);
});

test('refreshes the persisted event when an update callback has no previous snapshot', async () => {
	const previousEvent = calendarTestEvent('dragged-event', 'Previous title');
	const updatedEvent = calendarTestEvent('dragged-event', 'Updated title');
	const scenario = createPersistenceScenario([updatedEvent], () => [previousEvent]);

	await scenario.actions.saveUpdatedEvent(updatedEvent);

	expect(scenario.events().map((event) => event.title)).toEqual(['Previous title']);
	expect(scenario.refreshCount()).toBe(1);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
});

test('restores an optimistically deleted event after delete reports target unavailable', async () => {
	const persistedEvent = calendarTestEvent('deleted-event', 'Persisted title');
	const scenario = createPersistenceScenario([persistedEvent]);

	await scenario.actions.deleteEvent(persistedEvent.id);
	expect(scenario.events()).toEqual([]);

	await scenario.actions.flushPendingDelete();

	expect(scenario.events().map((event) => event.title)).toEqual(['Persisted title']);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
});

function createPersistenceScenario(
	initialEvents: DayFlowEvent[],
	refreshedEvents: () => DayFlowEvent[] = () => initialEvents,
	persistedOverrides: Partial<CalendarPersistedEventActions> = {},
	useDefaultNotifier = false
) {
	let calendarEvents = [...initialEvents];
	let calendarRefreshCount = 0;
	const notifications: string[] = [];
	const draftEvents = new CalendarDraftEventState(() => 'New Event', () => 'New Event');
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
		setIsSaving: () => {},
		setStatusMessage: () => {},
		setErrorMessage: () => {},
		openEventDetails: () => {},
		openMobileEventEditor: () => {},
		notifyEventsChanged: () => {},
		refreshCalendar: async () => {
			calendarRefreshCount += 1;
			calendarEvents = [...refreshedEvents()];
		},
		text: {
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
		deleteEventOnPageHide: () => {},
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
			showDeleteUndoToast: () => {},
			...(useDefaultNotifier ? {} : { notifyError: (message: string) => notifications.push(message) })
		}
	);
	return {
		actions,
		draftEvents,
		events: () => calendarEvents,
		notifications,
		refreshCount: () => calendarRefreshCount
	};
}

function calendarTestEvent(eventID: string, title: string): DayFlowEvent {
	return createEvent({
		id: eventID,
		title,
		start: new Date('2026-07-16T01:00:00Z'),
		end: new Date('2026-07-16T02:00:00Z'),
		allDay: false,
		calendarId: 'internkim'
	});
}

function targetUnavailableError(): CalendarPersistenceError {
	return new CalendarPersistenceError('calendar_target_unavailable', 'Could not persist the event.');
}

function calendarServerEvent(eventID: string, title: string): CalendarEvent {
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
