import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEventActionsContext } from './calendar-event-actions';
import {
	calendarDeleteUndoTimeoutMs,
	type showCalendarDeleteUndoToast
} from './calendar-delete-undo';
import { CalendarPersistenceError } from './calendar-event-persistence';
import { CalendarEventPersistenceOrder } from './calendar-event-persistence-order';
import type { CalendarPersistedEventActions } from './calendar-persisted-event-actions';

type CalendarDeletePersistenceOptions = {
	context: CalendarEventActionsContext;
	persistedEvents: CalendarPersistedEventActions;
	persistenceOrder: CalendarEventPersistenceOrder;
	refreshEventCountAfterRender: () => void;
	beginPersistence: () => void;
	finishPersistence: () => void;
	showPersistenceError: (error: unknown, fallback: string, versionConflictMessage?: string) => void;
	dismissUndoToast: () => void;
	showUndoToast: typeof showCalendarDeleteUndoToast;
};

export type CalendarDeletePersistenceActions = {
	deleteEvent: (eventID: string, revision: number) => Promise<void>;
	flushPendingDelete: () => Promise<void>;
	flushPendingDeleteOnPageHide: () => void;
};

export function createCalendarDeletePersistenceActions(
	options: CalendarDeletePersistenceOptions
): CalendarDeletePersistenceActions {
	let pendingDelete: {
		event: DayFlowEvent;
		expectedUpdatedAt: string | undefined;
		revision: number;
		timeoutID: ReturnType<typeof setTimeout>;
	} | null = null;

	async function deleteEvent(eventID: string, revision: number): Promise<void> {
		void persistPendingDelete(false);
		const event = options.context.getCalendarEvents().find((candidate) => candidate.id === eventID);
		if (!event) return;
		options.context.invalidatePendingEventLoad();
		options.context.removeCalendarEvent(eventID);
		options.refreshEventCountAfterRender();
		pendingDelete = {
			event,
			expectedUpdatedAt: typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined,
			revision,
			timeoutID: setTimeout(() => {
				void flushPendingDelete();
			}, calendarDeleteUndoTimeoutMs)
		};
		options.showUndoToast({
			message: options.context.text.deleteUndoMessage,
			actionLabel: options.context.text.deleteUndoAction,
			undo: () => undoPendingDelete(eventID)
		});
	}

	function undoPendingDelete(eventID: string): void {
		if (!pendingDelete || pendingDelete.event.id !== eventID) return;
		const event = pendingDelete.event;
		clearTimeout(pendingDelete.timeoutID);
		pendingDelete = null;
		const revision = options.persistenceOrder.beginAction(eventID);
		options.context.invalidatePendingEventLoad();
		if (!options.context.getCalendarEvents().some((candidate) => candidate.id === event.id)) {
			options.context.restoreCalendarEvent(event);
		}
		options.dismissUndoToast();
		options.refreshEventCountAfterRender();
		void options.persistenceOrder.runLatestAction(eventID, revision, async () => {
			await options.context.refreshCalendar();
			if (options.persistenceOrder.isLatestAction(eventID, revision)) {
				options.persistenceOrder.clearPersistedUpdatedAt(eventID);
			}
		});
	}

	async function flushPendingDelete(): Promise<void> {
		await persistPendingDelete(true);
	}

	function flushPendingDeleteOnPageHide(): void {
		const deleteToFlush = pendingDelete;
		if (!deleteToFlush) return;
		clearTimeout(deleteToFlush.timeoutID);
		pendingDelete = null;
		options.dismissUndoToast();
		void options.persistenceOrder.runLatestAction(
			deleteToFlush.event.id,
			deleteToFlush.revision,
			async () => {
				options.persistedEvents.deleteEventOnPageHide(
					deleteToFlush.event.id,
					deleteExpectedUpdatedAt(deleteToFlush)
				);
				options.persistenceOrder.clearPersistedUpdatedAt(deleteToFlush.event.id);
				options.context.notifyEventsChanged();
			}
		);
	}

	async function persistPendingDelete(shouldClearNotice: boolean): Promise<void> {
		const deleteToFlush = pendingDelete;
		if (!deleteToFlush) return;
		clearTimeout(deleteToFlush.timeoutID);
		pendingDelete = null;
		if (shouldClearNotice) options.dismissUndoToast();
		await options.persistenceOrder.runLatestAction(deleteToFlush.event.id, deleteToFlush.revision, async () => {
			options.beginPersistence();
			try {
				await options.persistedEvents.deleteEvent(
					deleteToFlush.event.id,
					deleteExpectedUpdatedAt(deleteToFlush)
				);
				if (!options.persistenceOrder.isLatestAction(deleteToFlush.event.id, deleteToFlush.revision)) return;
				options.persistenceOrder.clearPersistedUpdatedAt(deleteToFlush.event.id);
				options.context.invalidatePendingEventLoad();
				options.context.removeCalendarEvent(deleteToFlush.event.id);
				options.context.notifyEventsChanged();
				options.refreshEventCountAfterRender();
			} catch (error) {
				if (!options.persistenceOrder.isLatestAction(deleteToFlush.event.id, deleteToFlush.revision)) return;
				const isVersionConflict = error instanceof CalendarPersistenceError
					&& error.code === 'calendar_event_version_conflict';
				if (!isVersionConflict && !hasLocalEvent(deleteToFlush.event.id)) {
					options.context.restoreCalendarEvent(deleteToFlush.event);
				}
				if (isVersionConflict) {
					options.persistenceOrder.clearPersistedUpdatedAt(deleteToFlush.event.id);
				}
				options.showPersistenceError(
					error,
					options.context.text.deleteError,
					options.context.text.calendarDeleteVersionConflictError
				);
				await options.context.refreshCalendar();
				hidePendingDeleteAfterRefresh();
			} finally {
				options.finishPersistence();
			}
		});
	}

	function deleteExpectedUpdatedAt(deleteToFlush: NonNullable<typeof pendingDelete>): string | undefined {
		return options.persistenceOrder.persistedUpdatedAt(
			deleteToFlush.event.id,
			deleteToFlush.expectedUpdatedAt
		);
	}

	function hasLocalEvent(eventID: string): boolean {
		return options.context.getCalendarEvents().some((event) => event.id === eventID);
	}

	function hidePendingDeleteAfterRefresh(): void {
		if (!pendingDelete) return;
		options.context.removeCalendarEvent(pendingDelete.event.id);
		options.refreshEventCountAfterRender();
	}

	return {
		deleteEvent,
		flushPendingDelete,
		flushPendingDeleteOnPageHide
	};
}
