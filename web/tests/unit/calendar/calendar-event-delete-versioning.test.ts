import { expect, test } from 'bun:test';

import { CalendarPersistenceError } from '../../../src/routes/calendar/embed/calendar-event-persistence';
import {
	calendarServerEvent,
	calendarVersionedTestEvent,
	createPersistenceScenario,
	type CalendarEvent
} from './calendar-event-persistence-scenario';

const initialUpdatedAt = '2026-07-17T01:00:00Z';
const savedUpdatedAt = '2026-07-17T02:00:00Z';

test('normal delete uses the persisted version returned by a preceding PUT', async () => {
	const previousEvent = calendarVersionedTestEvent('normal-delete-version', 'Previous title', initialUpdatedAt);
	const updatedEvent = calendarVersionedTestEvent('normal-delete-version', 'Updated title', initialUpdatedAt);
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	let deleteExpectedUpdatedAt: string | undefined;
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([updatedEvent], () => [], {
		writeEvent: async () => {
			reportWriteStarted();
			return pendingWrite;
		},
		deleteEvent: async (_eventID, expectedUpdatedAt) => {
			deleteExpectedUpdatedAt = expectedUpdatedAt;
		}
	});

	const save = scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(updatedEvent.id);
	const flush = scenario.actions.flushPendingDelete();
	resolveWrite({
		...calendarServerEvent(updatedEvent.id, updatedEvent.title),
		updatedAt: savedUpdatedAt
	});
	await Promise.all([save, flush]);

	expect(deleteExpectedUpdatedAt).toBe(savedUpdatedAt);
});

test('pagehide delete waits for a preceding PUT and uses its persisted version', async () => {
	const previousEvent = calendarVersionedTestEvent('pagehide-delete-version', 'Previous title', initialUpdatedAt);
	const updatedEvent = calendarVersionedTestEvent('pagehide-delete-version', 'Updated title', initialUpdatedAt);
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	const pagehideDeleteVersions: Array<string | undefined> = [];
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([updatedEvent], () => [], {
		writeEvent: async () => {
			reportWriteStarted();
			return pendingWrite;
		},
		deleteEventOnPageHide: (_eventID, expectedUpdatedAt) => {
			pagehideDeleteVersions.push(expectedUpdatedAt);
		}
	});

	const save = scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(updatedEvent.id);
	scenario.actions.flushPendingDeleteOnPageHide();

	expect(pagehideDeleteVersions).toEqual([]);

	resolveWrite({
		...calendarServerEvent(updatedEvent.id, updatedEvent.title),
		updatedAt: savedUpdatedAt
	});
	await save;
	await waitForQueuedPersistence();

	expect(pagehideDeleteVersions).toEqual([savedUpdatedAt]);
});

test('delete version conflict refreshes without restoring the stale snapshot', async () => {
	const deletedEvent = calendarVersionedTestEvent('cross-tab-delete', 'Stale title', initialUpdatedAt);
	const remoteEvent = calendarVersionedTestEvent('cross-tab-delete', 'Remote title', savedUpdatedAt);
	const scenario = createPersistenceScenario([deletedEvent], () => [remoteEvent], {
		deleteEvent: async () => {
			throw new CalendarPersistenceError('calendar_event_version_conflict', 'Could not delete the event.');
		}
	});

	await scenario.actions.deleteEvent(deletedEvent.id);
	await scenario.actions.flushPendingDelete();

	expect(scenario.restoredEventTitles).toEqual([]);
	expect(scenario.events().map((event) => event.title)).toEqual(['Remote title']);
	expect(scenario.notifications).toEqual([
		'This event changed elsewhere, so it could not be deleted. The latest server version has been reloaded.'
	]);
	expect(scenario.refreshCount()).toBe(1);
});

async function waitForQueuedPersistence(): Promise<void> {
	await new Promise<void>((resolve) => setTimeout(resolve, 0));
}
