import { expect, test } from 'bun:test';

import {
	calendarPersistenceErrorFromResponse
} from '../../../src/routes/calendar/embed/calendar-event-persistence';

import {
	calendarServerEvent,
	calendarTestEvent,
	calendarVersionedTestEvent,
	createPersistenceScenario,
	targetUnavailableError,
	type CalendarEvent,
	targetUnavailableMessage
} from './calendar-event-persistence-scenario';

const initialUpdatedAt = '2026-07-17T01:00:00Z';

test('finalizes the undo window without issuing a direct DELETE', async () => {
	const persistedEvent = calendarVersionedTestEvent('intent-expiry', 'Persisted title', initialUpdatedAt);
	let intentCount = 0;
	let directDeleteCount = 0;
	const scenario = createPersistenceScenario([persistedEvent], () => [], {
		createDeleteIntent: async (_eventID, operationID) => {
			intentCount += 1;
			return { operationID, executeAt: '2026-07-17T01:00:05Z' };
		},
		deleteEvent: async () => {
			directDeleteCount += 1;
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	await scenario.actions.flushPendingDelete();
	await waitForQueuedPersistence();

	expect(intentCount).toBe(1);
	expect(directDeleteCount).toBe(0);
});

test('waits for a preceding PUT before recovering from delete intent registration failure', async () => {
	const previousEvent = calendarVersionedTestEvent('intent-failure-after-put', 'Previous title', initialUpdatedAt);
	const updatedEvent = calendarVersionedTestEvent('intent-failure-after-put', 'Updated title', initialUpdatedAt);
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([updatedEvent], () => [updatedEvent], {
		writeEvent: async () => {
			reportWriteStarted();
			return pendingWrite;
		},
		createDeleteIntent: async () => {
			throw targetUnavailableError();
		}
	});

	const save = scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(updatedEvent.id);
	await waitForQueuedPersistence();

	expect(scenario.restoredEventTitles).toEqual([]);
	expect(scenario.refreshCount()).toBe(0);

	resolveWrite(calendarServerEvent(updatedEvent.id, updatedEvent.title));
	await save;
	await waitForQueuedPersistence();

	expect(scenario.restoredEventTitles).toEqual(['Updated title']);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
	expect(scenario.refreshCount()).toBe(1);
});

test('starts Undo cancellation and restores the event before intent registration settles', async () => {
	const persistedEvent = calendarVersionedTestEvent('immediate-intent-cancel', 'Persisted title', initialUpdatedAt);
	let resolveCreate: (value: { operationID: string; executeAt: string }) => void = () => {};
	let isCreatePending = false;
	let cancelStartedWhileCreatePending = false;
	let createdOperationID = '';
	let canceledOperationID = '';
	let createdClientID = '';
	let canceledClientID = '';
	let createSequence = 0;
	let cancelSequence = 0;
	const pendingCreate = new Promise<{ operationID: string; executeAt: string }>((resolve) => {
		resolveCreate = resolve;
	});
	const scenario = createPersistenceScenario([persistedEvent], () => [persistedEvent], {
		createDeleteIntent: async (_eventID, operationID, clientID, sequence) => {
			isCreatePending = true;
			createdOperationID = operationID;
			createdClientID = clientID;
			createSequence = sequence;
			return pendingCreate;
		},
		cancelDeleteIntent: async (_eventID, operationID, clientID, sequence) => {
			cancelStartedWhileCreatePending = isCreatePending;
			canceledOperationID = operationID;
			canceledClientID = clientID;
			cancelSequence = sequence;
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	scenario.undoPendingDelete();

	expect(cancelStartedWhileCreatePending).toBe(true);
	expect(scenario.events().map((event) => event.id)).toEqual([persistedEvent.id]);
	expect(createdOperationID).not.toBe('');
	expect(canceledOperationID).toBe(createdOperationID);
	expect(canceledClientID).toBe(createdClientID);
	expect(cancelSequence).toBe(createSequence + 1);

	isCreatePending = false;
	resolveCreate({ operationID: createdOperationID, executeAt: '2026-07-17T01:00:05Z' });
	await waitForQueuedPersistence();
});

test('handles Undo cancellation failure without waiting for intent registration to settle', async () => {
	const persistedEvent = calendarVersionedTestEvent('independent-intent-cancel', 'Persisted title', initialUpdatedAt);
	const remoteEvent = calendarVersionedTestEvent('independent-intent-cancel', 'Remote title', '2026-07-17T02:00:00Z');
	let resolveCreate: (value: { operationID: string; executeAt: string }) => void = () => {};
	let operationID = '';
	const pendingCreate = new Promise<{ operationID: string; executeAt: string }>((resolve) => {
		resolveCreate = resolve;
	});
	const scenario = createPersistenceScenario([persistedEvent], () => [remoteEvent], {
		createDeleteIntent: async (_eventID, currentOperationID) => {
			operationID = currentOperationID;
			return pendingCreate;
		},
		cancelDeleteIntent: async () => {
			throw targetUnavailableError();
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	scenario.undoPendingDelete();
	await waitForQueuedPersistence();
	const refreshCountBeforeRegistrationSettled = scenario.refreshCount();
	resolveCreate({ operationID, executeAt: '2026-07-17T01:00:05Z' });
	await waitForQueuedPersistence();

	expect(refreshCountBeforeRegistrationSettled).toBe(1);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
});

test('refreshes the server event after Undo cancellation fails', async () => {
	const persistedEvent = calendarVersionedTestEvent('intent-cancel-failure', 'Persisted title', initialUpdatedAt);
	const remoteEvent = calendarVersionedTestEvent('intent-cancel-failure', 'Remote title', '2026-07-17T02:00:00Z');
	let cancelCount = 0;
	const scenario = createPersistenceScenario([persistedEvent], () => [remoteEvent], {
		createDeleteIntent: async (_eventID, operationID) => ({
			operationID,
			executeAt: '2026-07-17T01:00:05Z'
		}),
		cancelDeleteIntent: async () => {
			cancelCount += 1;
			throw targetUnavailableError();
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	scenario.undoPendingDelete();
	await waitForQueuedPersistence();

	expect(cancelCount).toBe(1);
	expect(scenario.events().map((event) => event.title)).toEqual(['Remote title']);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
	expect(scenario.refreshCount()).toBe(1);
});

test('reloads the latest server event after a delete intent cancellation conflict', async () => {
	const persistedEvent = calendarVersionedTestEvent('intent-cancel-conflict', 'Stale title', initialUpdatedAt);
	const remoteEvent = calendarVersionedTestEvent(
		'intent-cancel-conflict',
		'Remote title',
		'2026-07-17T02:00:00Z'
	);
	const conflictError = await calendarPersistenceErrorFromResponse(
		new Response(JSON.stringify({ code: 'calendar_delete_intent_conflict' }), {
			status: 409,
			headers: { 'Content-Type': 'application/json' }
		}),
		'Could not delete the event.'
	);
	const scenario = createPersistenceScenario([persistedEvent], () => [remoteEvent], {
		createDeleteIntent: async (_eventID, operationID) => ({
			operationID,
			executeAt: '2026-07-17T01:00:05Z'
		}),
		cancelDeleteIntent: async () => {
			throw conflictError;
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	scenario.undoPendingDelete();
	await waitForQueuedPersistence();

	expect(conflictError.code).toBe('calendar_delete_intent_conflict');
	expect(scenario.refreshCount()).toBe(1);
	expect(scenario.events().map((event) => event.title)).toEqual(['Remote title']);
	expect(scenario.notifications).toEqual(['Could not delete the event.']);
});

test('refreshes the server value after undo follows a failed pending update', async () => {
	const serverEvent = calendarTestEvent('failed-update-undo', 'Server title');
	const optimisticEvent = calendarTestEvent('failed-update-undo', 'Optimistic title');
	let rejectWrite: (error: Error) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((_, reject) => {
		rejectWrite = reject;
	});
	const scenario = createPersistenceScenario([optimisticEvent], () => [serverEvent], {
		writeEvent: async () => {
			reportWriteStarted();
			return pendingWrite;
		}
	});

	const save = scenario.actions.saveUpdatedEvent(optimisticEvent, serverEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(optimisticEvent.id);
	scenario.undoPendingDelete();
	rejectWrite(targetUnavailableError());
	await save;
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.title)).toEqual(['Server title']);
	expect(scenario.refreshCount()).toBe(1);
});

test('refreshes the saved value after undo follows a successful pending update', async () => {
	const serverEvent = calendarTestEvent('successful-update-undo', 'Server title');
	const optimisticEvent = calendarTestEvent('successful-update-undo', 'Optimistic title');
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	let refreshedEvents = [serverEvent];
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([optimisticEvent], () => refreshedEvents, {
		writeEvent: async () => {
			reportWriteStarted();
			return pendingWrite;
		}
	});

	const save = scenario.actions.saveUpdatedEvent(optimisticEvent, serverEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(optimisticEvent.id);
	scenario.undoPendingDelete();
	refreshedEvents = [optimisticEvent];
	resolveWrite(calendarServerEvent(optimisticEvent.id, optimisticEvent.title));
	await save;
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.title)).toEqual(['Optimistic title']);
	expect(scenario.refreshCount()).toBe(1);
});

test('skips an undo refresh when a newer update is queued', async () => {
	const serverEvent = calendarTestEvent('undo-newer-update', 'Server title');
	const firstUpdate = calendarTestEvent('undo-newer-update', 'First update');
	const latestUpdate = calendarTestEvent('undo-newer-update', 'Latest update');
	let rejectFirstWrite: (error: Error) => void = () => {};
	let reportFirstWriteStarted: () => void = () => {};
	let writeCount = 0;
	const firstWriteStarted = new Promise<void>((resolve) => {
		reportFirstWriteStarted = resolve;
	});
	const firstWrite = new Promise<CalendarEvent>((_, reject) => {
		rejectFirstWrite = reject;
	});
	const scenario = createPersistenceScenario([firstUpdate], () => [serverEvent], {
		writeEvent: async (_path, _method, event) => {
			writeCount += 1;
			if (writeCount === 1) {
				reportFirstWriteStarted();
				return firstWrite;
			}
			return calendarServerEvent(event.id, event.title ?? '');
		}
	});

	const firstSave = scenario.actions.saveUpdatedEvent(firstUpdate, serverEvent);
	await firstWriteStarted;
	await scenario.actions.deleteEvent(firstUpdate.id);
	scenario.undoPendingDelete();
	scenario.context.restoreCalendarEvent(latestUpdate);
	const latestSave = scenario.actions.saveUpdatedEvent(latestUpdate, firstUpdate);
	rejectFirstWrite(targetUnavailableError());
	await Promise.all([firstSave, latestSave]);
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.title)).toEqual(['Latest update']);
	expect(scenario.refreshCount()).toBe(0);
});

test('invalidates a pending load before undo restores a deleted event', async () => {
	const persistedEvent = calendarTestEvent('undo-delete-event', 'Persisted title');
	const scenario = createPersistenceScenario([persistedEvent]);

	await scenario.actions.deleteEvent(persistedEvent.id);
	scenario.context.restoreCalendarEvent(persistedEvent);
	scenario.undoPendingDelete();

	expect(scenario.events().map((event) => event.id)).toEqual([persistedEvent.id]);
	expect(scenario.pendingLoadInvalidationCount()).toBe(2);
});

test('removes a stale refresh result after delete intent registration succeeds', async () => {
	const persistedEvent = calendarVersionedTestEvent('delete-refresh-race', 'Persisted title', initialUpdatedAt);
	let resolveIntent: (value: { operationID: string; executeAt: string }) => void = () => {};
	let reportIntentStarted: () => void = () => {};
	let operationID = '';
	const intentStarted = new Promise<void>((resolve) => {
		reportIntentStarted = resolve;
	});
	const pendingIntent = new Promise<{ operationID: string; executeAt: string }>((resolve) => {
		resolveIntent = resolve;
	});
	const scenario = createPersistenceScenario([persistedEvent], () => [], {
		createDeleteIntent: async (_eventID, currentOperationID) => {
			operationID = currentOperationID;
			reportIntentStarted();
			return pendingIntent;
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	await intentStarted;
	scenario.context.restoreCalendarEvent(persistedEvent);
	resolveIntent({ operationID, executeAt: '2026-07-17T01:00:05Z' });
	await waitForQueuedPersistence();

	expect(scenario.events()).toEqual([]);
	expect(scenario.pendingLoadInvalidationCount()).toBe(2);
});

test('keeps a newer local update when an older delete intent registration succeeds', async () => {
	const persistedEvent = calendarVersionedTestEvent('delete-newer-update-race', 'Persisted title', initialUpdatedAt);
	const updatedEvent = calendarVersionedTestEvent('delete-newer-update-race', 'Newer title', initialUpdatedAt);
	let resolveIntent: (value: { operationID: string; executeAt: string }) => void = () => {};
	let reportIntentStarted: () => void = () => {};
	let operationID = '';
	const intentStarted = new Promise<void>((resolve) => {
		reportIntentStarted = resolve;
	});
	const pendingIntent = new Promise<{ operationID: string; executeAt: string }>((resolve) => {
		resolveIntent = resolve;
	});
	const scenario = createPersistenceScenario([persistedEvent], () => [], {
		createDeleteIntent: async (_eventID, currentOperationID) => {
			operationID = currentOperationID;
			reportIntentStarted();
			return pendingIntent;
		},
		writeEvent: async (_path, _method, event) => calendarServerEvent(event.id, event.title ?? '')
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	await intentStarted;
	scenario.context.restoreCalendarEvent(updatedEvent);
	const update = scenario.actions.saveUpdatedEvent(updatedEvent, persistedEvent);
	resolveIntent({ operationID, executeAt: '2026-07-17T01:00:05Z' });
	await update;
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.title)).toEqual(['Newer title']);
	expect(scenario.pendingLoadInvalidationCount()).toBe(2);
});

test('restores an optimistically deleted event after delete reports target unavailable', async () => {
	const persistedEvent = calendarTestEvent('deleted-event', 'Persisted title');
	const scenario = createPersistenceScenario([persistedEvent]);

	await scenario.actions.deleteEvent(persistedEvent.id);
	expect(scenario.events()).toEqual([]);

	await scenario.actions.flushPendingDelete();
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.title)).toEqual(['Persisted title']);
	expect(scenario.notifications).toEqual([targetUnavailableMessage]);
});

async function waitForQueuedPersistence(): Promise<void> {
	await new Promise<void>((resolve) => setTimeout(resolve, 0));
}
