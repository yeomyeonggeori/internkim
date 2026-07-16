import { expect, test } from 'bun:test';

import {
	calendarServerEvent,
	calendarTestEvent,
	createEventActionsForScenario,
	createPersistenceScenario,
	targetUnavailableError,
	targetUnavailableMessage,
	type CalendarEvent
} from './calendar-event-persistence-scenario';

test('invalidates a pending load before adding a quick draft event', () => {
	const scenario = createPersistenceScenario([]);
	const actions = createEventActionsForScenario(scenario);

	actions.createQuickEvent();

	expect(scenario.pendingLoadInvalidationCount()).toBe(1);
});

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

test('serializes a pending draft create and persists the latest edit', async () => {
	const originalDraft = calendarTestEvent('pending-edit-event', '');
	const firstDraft = calendarTestEvent('pending-edit-event', 'First title');
	const latestDraft = calendarTestEvent('pending-edit-event', 'Latest title');
	const writes: Array<{ method: 'POST' | 'PUT'; title: string; expectedUpdatedAt?: string }> = [];
	let resolveFirstWrite: (event: CalendarEvent) => void = () => {};
	const firstWrite = new Promise<CalendarEvent>((resolve) => {
		resolveFirstWrite = resolve;
	});
	const scenario = createPersistenceScenario([latestDraft], () => [], {
		writeEvent: async (_path, method, event, expectedUpdatedAt) => {
			writes.push({ method, title: event.title ?? '', expectedUpdatedAt });
			if (writes.length === 1) return firstWrite;
			return calendarServerEvent(event.id, event.title ?? '');
		}
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const firstSave = scenario.actions.saveUpdatedEvent(firstDraft);
	await Promise.resolve();
	await scenario.actions.saveUpdatedEvent(latestDraft);

	expect(writes).toEqual([{ method: 'POST', title: 'First title', expectedUpdatedAt: undefined }]);
	expect(scenario.draftEvents.hasPendingCreate(firstDraft.id)).toBe(true);

	resolveFirstWrite(calendarServerEvent(firstDraft.id, firstDraft.title));
	await firstSave;

	expect(writes).toEqual([
		{ method: 'POST', title: 'First title', expectedUpdatedAt: undefined },
		{
			method: 'PUT',
			title: 'Latest title',
			expectedUpdatedAt: '2026-07-16T00:00:00Z'
		}
	]);
	expect(scenario.draftEvents.hasPendingCreate(firstDraft.id)).toBe(false);
	expect(scenario.draftEvents.isDraftEvent(firstDraft.id)).toBe(false);
});

test('deletes an event created on the server after the pending draft was deleted', async () => {
	const originalDraft = calendarTestEvent('pending-delete-event', '');
	const titledDraft = calendarTestEvent('pending-delete-event', 'Draft title');
	const deletedEvents: Array<{ eventID: string; expectedUpdatedAt?: string }> = [];
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([titledDraft], () => [], {
		writeEvent: async () => pendingWrite,
		deleteEvent: async (eventID, expectedUpdatedAt) => {
			deletedEvents.push({ eventID, expectedUpdatedAt });
		}
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const savePromise = scenario.actions.saveUpdatedEvent(titledDraft);
	await scenario.actions.deleteEvent(titledDraft.id);
	resolveWrite(calendarServerEvent(titledDraft.id, titledDraft.title));
	await savePromise;

	expect(scenario.events()).toEqual([]);
	expect(scenario.draftEvents.isDraftEvent(titledDraft.id)).toBe(false);
	expect(deletedEvents).toEqual([
		{ eventID: titledDraft.id, expectedUpdatedAt: '2026-07-16T00:00:00Z' }
	]);
});

test('removes a stale refresh result after pending draft server delete succeeds', async () => {
	const originalDraft = calendarTestEvent('pending-draft-delete-refresh-race', '');
	const titledDraft = calendarTestEvent('pending-draft-delete-refresh-race', 'Draft title');
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	let resolveDelete: () => void = () => {};
	let reportDeleteStarted: () => void = () => {};
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const deleteStarted = new Promise<void>((resolve) => {
		reportDeleteStarted = resolve;
	});
	const pendingDelete = new Promise<void>((resolve) => {
		resolveDelete = resolve;
	});
	const scenario = createPersistenceScenario([titledDraft], () => [], {
		writeEvent: async () => pendingWrite,
		deleteEvent: async () => {
			reportDeleteStarted();
			await pendingDelete;
		}
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const save = scenario.actions.saveUpdatedEvent(titledDraft);
	await scenario.actions.deleteEvent(titledDraft.id);
	resolveWrite(calendarServerEvent(titledDraft.id, titledDraft.title));
	await deleteStarted;
	scenario.context.restoreCalendarEvent(titledDraft);
	resolveDelete();
	await save;

	expect(scenario.events()).toEqual([]);
	expect(scenario.pendingLoadInvalidationCount()).toBe(3);
});

test('deletes the server event when a newer draft update fails after local deletion', async () => {
	const originalDraft = calendarTestEvent('pending-update-failure-delete', '');
	const firstDraft = calendarTestEvent('pending-update-failure-delete', 'First title');
	const latestDraft = calendarTestEvent('pending-update-failure-delete', 'Latest title');
	const deletedEvents: Array<{ eventID: string; expectedUpdatedAt: string | undefined }> = [];
	let resolveCreate: (event: CalendarEvent) => void = () => {};
	let rejectUpdate: (error: Error) => void = () => {};
	let reportUpdateStarted: () => void = () => {};
	const pendingCreate = new Promise<CalendarEvent>((resolve) => {
		resolveCreate = resolve;
	});
	const updateStarted = new Promise<void>((resolve) => {
		reportUpdateStarted = resolve;
	});
	const pendingUpdate = new Promise<CalendarEvent>((_, reject) => {
		rejectUpdate = reject;
	});
	const scenario = createPersistenceScenario([latestDraft], () => [], {
		writeEvent: async (_path, method) => {
			if (method === 'POST') return pendingCreate;
			reportUpdateStarted();
			return pendingUpdate;
		},
		deleteEvent: async (eventID, expectedUpdatedAt) => {
			deletedEvents.push({ eventID, expectedUpdatedAt });
		}
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const savePromise = scenario.actions.saveUpdatedEvent(firstDraft);
	await Promise.resolve();
	await scenario.actions.saveUpdatedEvent(latestDraft);
	resolveCreate(calendarServerEvent(firstDraft.id, firstDraft.title));
	await updateStarted;
	await scenario.actions.deleteEvent(latestDraft.id);
	rejectUpdate(targetUnavailableError());
	await savePromise;

	expect(scenario.events()).toEqual([]);
	expect(scenario.draftEvents.isDraftEvent(latestDraft.id)).toBe(false);
	expect(deletedEvents).toEqual([
		{ eventID: latestDraft.id, expectedUpdatedAt: '2026-07-16T00:00:00Z' }
	]);
	expect(scenario.notifications).toEqual([]);
	expect(scenario.savingStates).toEqual([true, false]);
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

test('does not refresh a draft deleted while server metadata is being applied', async () => {
	const originalDraft = calendarTestEvent('draft-metadata-delete', '');
	const titledDraft = calendarTestEvent('draft-metadata-delete', 'Draft title');
	let rejectMetadata: (error: Error) => void = () => {};
	let reportMetadataStarted: () => void = () => {};
	const metadataStarted = new Promise<void>((resolve) => {
		reportMetadataStarted = resolve;
	});
	const pendingMetadata = new Promise<void>((_, reject) => {
		rejectMetadata = reject;
	});
	const scenario = createPersistenceScenario([titledDraft], () => [titledDraft], {
		writeEvent: async (_path, _method, event) => calendarServerEvent(event.id, event.title ?? ''),
		applyServerMetadata: async () => {
			reportMetadataStarted();
			await pendingMetadata;
		},
		deleteEvent: async () => {}
	});
	scenario.draftEvents.addCreatedEvent(originalDraft);

	const savePromise = scenario.actions.saveUpdatedEvent(titledDraft);
	await metadataStarted;
	await scenario.actions.deleteEvent(titledDraft.id);
	rejectMetadata(new Error('stale draft metadata failure'));
	await savePromise;
	await scenario.actions.flushPendingDelete();

	expect(scenario.events()).toEqual([]);
	expect(scenario.refreshCount()).toBe(0);
	expect(scenario.notifications).toEqual([]);
});
