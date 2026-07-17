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

test('starts a higher-sequence delete intent while the preceding PUT is pending', async () => {
	const previousEvent = calendarVersionedTestEvent('immediate-delete-intent', 'Previous title', initialUpdatedAt);
	const updatedEvent = calendarVersionedTestEvent('immediate-delete-intent', 'Updated title', initialUpdatedAt);
	let resolveWrite: (event: CalendarEvent) => void = () => {};
	let reportWriteStarted: () => void = () => {};
	let isWritePending = true;
	let intentStartedWhileWritePending = false;
	let writeClientID: string | undefined;
	let writeSequence: number | undefined;
	let intentClientID: string | undefined;
	let intentSequence: number | undefined;
	let intentExpectedUpdatedAt: string | undefined;
	const writeStarted = new Promise<void>((resolve) => {
		reportWriteStarted = resolve;
	});
	const pendingWrite = new Promise<CalendarEvent>((resolve) => {
		resolveWrite = resolve;
	});
	const scenario = createPersistenceScenario([updatedEvent], () => [], {
		writeEvent: async (_path, _method, _event, _expectedUpdatedAt, mutationClientID, mutationSequence) => {
			writeClientID = mutationClientID;
			writeSequence = mutationSequence;
			reportWriteStarted();
			return pendingWrite;
		},
		createDeleteIntent: async (_eventID, operationID, clientID, sequence, expectedUpdatedAt) => {
			intentStartedWhileWritePending = isWritePending;
			intentClientID = clientID;
			intentSequence = sequence;
			intentExpectedUpdatedAt = expectedUpdatedAt;
			return { operationID, executeAt: '2026-07-17T01:00:05Z' };
		}
	});

	const save = scenario.actions.saveUpdatedEvent(updatedEvent, previousEvent);
	await writeStarted;
	await scenario.actions.deleteEvent(updatedEvent.id);
	isWritePending = false;
	resolveWrite({
		...calendarServerEvent(updatedEvent.id, updatedEvent.title),
		updatedAt: savedUpdatedAt
	});
	await save;
	await waitForQueuedPersistence();

	expect(intentStartedWhileWritePending).toBe(true);
	expect(typeof writeClientID).toBe('string');
	expect(intentClientID).toBe(writeClientID);
	expect(intentSequence).toBe((writeSequence ?? 0) + 1);
	expect(intentExpectedUpdatedAt).toBe(initialUpdatedAt);
});

test('delete intent version conflict refreshes without restoring the stale snapshot', async () => {
	const deletedEvent = calendarVersionedTestEvent('cross-tab-delete', 'Stale title', initialUpdatedAt);
	const remoteEvent = calendarVersionedTestEvent('cross-tab-delete', 'Remote title', savedUpdatedAt);
	const scenario = createPersistenceScenario([deletedEvent], () => [remoteEvent], {
		createDeleteIntent: async () => {
			throw new CalendarPersistenceError('calendar_event_version_conflict', 'Could not delete the event.');
		}
	});

	await scenario.actions.deleteEvent(deletedEvent.id);
	await waitForQueuedPersistence();

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
