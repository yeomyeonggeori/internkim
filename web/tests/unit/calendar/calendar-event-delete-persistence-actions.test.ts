import { expect, test } from 'bun:test';

import {
	calendarTestEvent,
	createPersistenceScenario,
	deleteFailureMessage,
	unknownPersistenceError
} from './calendar-event-persistence-scenario';

test('hides the event at once and removes it from the record when the undo window closes', async () => {
	const persistedEvent = calendarTestEvent('undo-window-expiry', 'Persisted title');
	const deleted: string[] = [];
	const scenario = createPersistenceScenario([persistedEvent], () => [], {
		deleteEvent: async (eventID: string) => {
			deleted.push(eventID);
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	expect(scenario.events()).toEqual([]);
	expect(deleted).toEqual([]);

	await scenario.actions.flushPendingDelete();
	await waitForQueuedPersistence();

	expect(deleted).toEqual([persistedEvent.id]);
});

test('undo inside the window puts the event back and never reaches the record', async () => {
	const persistedEvent = calendarTestEvent('undo-window-undo', 'Persisted title');
	const deleted: string[] = [];
	const scenario = createPersistenceScenario([persistedEvent], () => [persistedEvent], {
		deleteEvent: async (eventID: string) => {
			deleted.push(eventID);
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	scenario.undoPendingDelete();
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.id)).toEqual([persistedEvent.id]);
	expect(deleted).toEqual([]);
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

test('restores an optimistically deleted event after the record refuses the delete', async () => {
	const persistedEvent = calendarTestEvent('deleted-event', 'Persisted title');
	const scenario = createPersistenceScenario([persistedEvent], () => [persistedEvent], {
		deleteEvent: async () => {
			throw unknownPersistenceError();
		}
	});

	await scenario.actions.deleteEvent(persistedEvent.id);
	expect(scenario.events()).toEqual([]);

	await scenario.actions.flushPendingDelete();
	await waitForQueuedPersistence();

	expect(scenario.events().map((event) => event.title)).toEqual(['Persisted title']);
	expect(scenario.notifications).toEqual([deleteFailureMessage]);
});

test('a second delete closes the first undo window, so only one is ever open', async () => {
	const first = calendarTestEvent('first-delete', 'First');
	const second = calendarTestEvent('second-delete', 'Second');
	const scenario = createPersistenceScenario([first, second], () => [], {
		deleteEvent: async () => {}
	});

	await scenario.actions.deleteEvent(first.id);
	await scenario.actions.deleteEvent(second.id);

	expect(scenario.actions.undoLastDelete()).toBe(true);
	expect(scenario.actions.undoLastDelete()).toBe(false);
});

async function waitForQueuedPersistence(): Promise<void> {
	await new Promise<void>((resolve) => setTimeout(resolve, 0));
}
