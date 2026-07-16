import { expect, test } from 'bun:test';

import { CalendarPersistenceError } from '../../../src/routes/calendar/embed/calendar-event-persistence';

import {
	calendarServerEvent,
	calendarTestEvent,
	calendarVersionedTestEvent,
	createPersistenceScenario,
	targetUnavailableError,
	targetUnavailableMessage,
	toastErrorMessages,
	type CalendarEvent
} from './calendar-event-persistence-scenario';

test('invalidates pending loads before update delete and draft persistence callbacks', async () => {
	const persistedEvent = calendarTestEvent('local-mutation-event', 'Updated title');
	const draftEvent = calendarTestEvent('local-draft-event', '');
	const scenario = createPersistenceScenario([persistedEvent]);

	await scenario.actions.saveUpdatedEvent(persistedEvent);
	await scenario.actions.deleteEvent(persistedEvent.id);
	await scenario.actions.saveCreatedEvent(draftEvent);

	expect(scenario.pendingLoadInvalidationCount()).toBe(3);
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

test('does not let a stale update failure overwrite a newer update', async () => {
	const previousEvent = calendarTestEvent('overlapping-update', 'Previous title');
	const firstUpdate = calendarTestEvent('overlapping-update', 'First update');
	const latestUpdate = calendarTestEvent('overlapping-update', 'Latest update');
	const writes: string[] = [];
	let rejectFirstWrite: (error: Error) => void = () => {};
	let reportFirstWriteStarted: () => void = () => {};
	const firstWriteStarted = new Promise<void>((resolve) => {
		reportFirstWriteStarted = resolve;
	});
	const firstWrite = new Promise<CalendarEvent>((_, reject) => {
		rejectFirstWrite = reject;
	});
	const scenario = createPersistenceScenario([latestUpdate], () => [previousEvent], {
		writeEvent: async (_path, _method, event) => {
			writes.push(event.title ?? '');
			if (writes.length === 1) {
				reportFirstWriteStarted();
				return firstWrite;
			}
			return calendarServerEvent(event.id, event.title ?? '');
		}
	});

	const firstSave = scenario.actions.saveUpdatedEvent(firstUpdate, previousEvent);
	await firstWriteStarted;
	const latestSave = scenario.actions.saveUpdatedEvent(latestUpdate, firstUpdate);
	rejectFirstWrite(targetUnavailableError());
	await Promise.all([firstSave, latestSave]);

	expect(writes).toEqual(['First update', 'Latest update']);
	expect(scenario.events().map((event) => event.title)).toEqual(['Latest update']);
	expect(scenario.notifications).toEqual([]);
});

test('uses the persisted version from a completed PUT for the next queued PUT', async () => {
	const initialUpdatedAt = '2026-07-17T01:00:00Z';
	const firstSavedUpdatedAt = '2026-07-17T02:00:00Z';
	const latestSavedUpdatedAt = '2026-07-17T03:00:00Z';
	const previousEvent = calendarVersionedTestEvent('overlapping-update-version', 'Previous title', initialUpdatedAt);
	const firstUpdate = calendarVersionedTestEvent('overlapping-update-version', 'First update', initialUpdatedAt);
	const latestUpdate = calendarVersionedTestEvent('overlapping-update-version', 'Latest update', initialUpdatedAt);
	const expectedUpdatedAtValues: Array<string | undefined> = [];
	let resolveFirstWrite: (event: CalendarEvent) => void = () => {};
	let reportFirstWriteStarted: () => void = () => {};
	const firstWriteStarted = new Promise<void>((resolve) => {
		reportFirstWriteStarted = resolve;
	});
	const firstWrite = new Promise<CalendarEvent>((resolve) => {
		resolveFirstWrite = resolve;
	});
	const scenario = createPersistenceScenario([latestUpdate], () => [previousEvent], {
		writeEvent: async (_path, _method, event, expectedUpdatedAt) => {
			expectedUpdatedAtValues.push(expectedUpdatedAt);
			if (expectedUpdatedAtValues.length === 1) {
				reportFirstWriteStarted();
				return firstWrite;
			}
			return {
				...calendarServerEvent(event.id, event.title ?? ''),
				updatedAt: latestSavedUpdatedAt
			};
		}
	});

	const firstSave = scenario.actions.saveUpdatedEvent(firstUpdate, previousEvent);
	await firstWriteStarted;
	const latestSave = scenario.actions.saveUpdatedEvent(latestUpdate, firstUpdate);
	resolveFirstWrite({
		...calendarServerEvent(firstUpdate.id, firstUpdate.title),
		updatedAt: firstSavedUpdatedAt
	});
	await Promise.all([firstSave, latestSave]);

	expect(expectedUpdatedAtValues).toEqual([initialUpdatedAt, firstSavedUpdatedAt]);
});

test('refreshes from the server when metadata application fails after a successful update', async () => {
	const previousEvent = calendarTestEvent('metadata-failure', 'Previous title');
	const updatedEvent = calendarTestEvent('metadata-failure', 'Updated title');
	const serverEvent = calendarTestEvent('metadata-failure', 'Server title');
	const scenario = createPersistenceScenario([updatedEvent], () => [serverEvent], {
		writeEvent: async (_path, _method, event) => calendarServerEvent(event.id, event.title ?? ''),
		applyServerMetadata: async () => {
			throw new Error('event disappeared during metadata application');
		}
	});

	await scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);

	expect(scenario.events().map((event) => event.title)).toEqual(['Server title']);
	expect(scenario.refreshCount()).toBe(1);
	expect(scenario.notifications).toEqual(['Could not save the event.']);
});

test('does not refresh a deleted event after stale metadata application fails', async () => {
	const previousEvent = calendarTestEvent('stale-metadata-delete', 'Previous title');
	const updatedEvent = calendarTestEvent('stale-metadata-delete', 'Updated title');
	let rejectMetadata: (error: Error) => void = () => {};
	let reportMetadataStarted: () => void = () => {};
	const metadataStarted = new Promise<void>((resolve) => {
		reportMetadataStarted = resolve;
	});
	const pendingMetadata = new Promise<void>((_, reject) => {
		rejectMetadata = reject;
	});
	const scenario = createPersistenceScenario([updatedEvent], () => [previousEvent], {
		writeEvent: async (_path, _method, event) => calendarServerEvent(event.id, event.title ?? ''),
		applyServerMetadata: async () => {
			reportMetadataStarted();
			await pendingMetadata;
		},
		deleteEvent: async () => {}
	});

	const save = scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	await metadataStarted;
	await scenario.actions.deleteEvent(updatedEvent.id);
	rejectMetadata(new Error('stale metadata failure'));
	await save;
	await scenario.actions.flushPendingDelete();

	expect(scenario.events()).toEqual([]);
	expect(scenario.refreshCount()).toBe(0);
	expect(scenario.notifications).toEqual([]);
});

test('does not restore an event deleted while its update is pending', async () => {
	const previousEvent = calendarTestEvent('pending-update-delete', 'Previous title');
	const updatedEvent = calendarTestEvent('pending-update-delete', 'Updated title');
	let rejectWrite: (error: Error) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((_, reject) => {
		rejectWrite = reject;
	});
	const scenario = createPersistenceScenario([updatedEvent], () => [previousEvent], {
		writeEvent: async () => {
			reportWriteStarted();
			return pendingWrite;
		},
		deleteEvent: async () => {}
	});

	const save = scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(updatedEvent.id);
	rejectWrite(targetUnavailableError());
	await save;
	await scenario.actions.flushPendingDelete();

	expect(scenario.events()).toEqual([]);
	expect(scenario.notifications).toEqual([]);
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

test('shows a localized version conflict and refreshes the server event', async () => {
	const previousEvent = calendarTestEvent('version-conflict-event', 'Previous title');
	const updatedEvent = calendarTestEvent('version-conflict-event', 'Optimistic title');
	const serverEvent = calendarTestEvent('version-conflict-event', 'Server title');
	const scenario = createPersistenceScenario([updatedEvent], () => [serverEvent], {
		writeEvent: async () => {
			throw new CalendarPersistenceError('calendar_event_version_conflict', 'Could not save the event.');
		}
	});

	await scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);

	expect(scenario.events().map((event) => event.title)).toEqual(['Server title']);
	expect(scenario.notifications).toEqual([
		'This event changed elsewhere. The latest server version has been reloaded.'
	]);
	expect(scenario.refreshCount()).toBe(1);
});

test('releases the saving state when version conflict refresh fails', async () => {
	const previousEvent = calendarTestEvent('version-conflict-refresh-failure', 'Previous title');
	const updatedEvent = calendarTestEvent('version-conflict-refresh-failure', 'Optimistic title');
	const refreshError = new Error('calendar refresh failed');
	const scenario = createPersistenceScenario(
		[updatedEvent],
		() => {
			throw refreshError;
		},
		{
			writeEvent: async () => {
				throw new CalendarPersistenceError('calendar_event_version_conflict', 'Could not save the event.');
			}
		}
	);
	let rejection: unknown;

	try {
		await scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	} catch (error: unknown) {
		rejection = error;
	}

	expect(rejection).toBe(refreshError);
	expect(scenario.refreshCount()).toBe(1);
	expect(scenario.savingStates).toEqual([true, false]);
});
