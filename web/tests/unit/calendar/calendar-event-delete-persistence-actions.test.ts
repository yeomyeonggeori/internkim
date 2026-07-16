import { expect, test } from 'bun:test';

import {
	calendarServerEvent,
	calendarTestEvent,
	createPersistenceScenario,
	targetUnavailableError,
	type CalendarEvent,
	targetUnavailableMessage
} from './calendar-event-persistence-scenario';

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

test('removes a stale refresh result after persisted delete succeeds', async () => {
	const persistedEvent = calendarTestEvent('delete-refresh-race', 'Persisted title');
	let resolveDelete: () => void = () => {};
	let reportDeleteStarted: () => void = () => {};
	const deleteStarted = new Promise<void>((resolve) => {
		reportDeleteStarted = resolve;
	});
	const pendingDelete = new Promise<void>((resolve) => {
		resolveDelete = resolve;
	});
	const scenario = createPersistenceScenario([persistedEvent], () => [], {
		deleteEvent: async () => {
			reportDeleteStarted();
			await pendingDelete;
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	const flush = scenario.actions.flushPendingDelete();
	await deleteStarted;
	scenario.context.restoreCalendarEvent(persistedEvent);
	resolveDelete();
	await flush;

	expect(scenario.events()).toEqual([]);
	expect(scenario.pendingLoadInvalidationCount()).toBe(2);
});

test('keeps a newer local update when an older persisted delete succeeds', async () => {
	const persistedEvent = calendarTestEvent('delete-newer-update-race', 'Persisted title');
	const updatedEvent = calendarTestEvent('delete-newer-update-race', 'Newer title');
	let resolveDelete: () => void = () => {};
	let reportDeleteStarted: () => void = () => {};
	const deleteStarted = new Promise<void>((resolve) => {
		reportDeleteStarted = resolve;
	});
	const pendingDelete = new Promise<void>((resolve) => {
		resolveDelete = resolve;
	});
	const scenario = createPersistenceScenario([persistedEvent], () => [], {
		deleteEvent: async () => {
			reportDeleteStarted();
			await pendingDelete;
		},
		writeEvent: async (_path, _method, event) => calendarServerEvent(event.id, event.title ?? '')
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	const flush = scenario.actions.flushPendingDelete();
	await deleteStarted;
	scenario.context.restoreCalendarEvent(updatedEvent);
	const update = scenario.actions.saveUpdatedEvent(updatedEvent, persistedEvent);
	resolveDelete();
	await Promise.all([flush, update]);

	expect(scenario.events().map((event) => event.title)).toEqual(['Newer title']);
	expect(scenario.pendingLoadInvalidationCount()).toBe(2);
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

async function waitForQueuedPersistence(): Promise<void> {
	await new Promise<void>((resolve) => setTimeout(resolve, 0));
}
