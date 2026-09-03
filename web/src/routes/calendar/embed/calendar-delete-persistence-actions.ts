import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import type { CalendarEventActionsContext } from './calendar-event-actions';
import {
	calendarDeleteUndoTimeoutMs,
	type showCalendarDeleteUndoToast
} from './calendar-delete-undo';
import { CalendarEventPersistenceOrder } from './calendar-event-persistence-order';
import { calendarEventVersionConflictErrorCode } from './calendar-event-persistence';
import type { CalendarPersistedEventActions } from './calendar-persisted-event-actions';
import { isRefusalCode } from '$lib/public-api-call';

type CalendarDeletePersistenceOptions = {
	context: CalendarEventActionsContext;
	persistedEvents: CalendarPersistedEventActions;
	persistenceOrder: CalendarEventPersistenceOrder;
	refreshEventCountAfterRender: () => void;
	beginPersistence: () => void;
	finishPersistence: () => void;
	showPersistenceError: (message: string) => void;
	dismissUndoToast: () => void;
	showUndoToast: typeof showCalendarDeleteUndoToast;
};

export type CalendarDeletePersistenceActions = {
	deleteEvent: (eventID: string) => Promise<void>;
	flushPendingDelete: () => Promise<void>;
	undoLastDelete: () => boolean;
};

type PendingCalendarDelete = {
	event: DayTaskEvent;
	timeoutID: ReturnType<typeof setTimeout>;
};

export function createCalendarDeletePersistenceActions(
	options: CalendarDeletePersistenceOptions
): CalendarDeletePersistenceActions {
	let pendingDelete: PendingCalendarDelete | null = null;

	async function deleteEvent(eventID: string): Promise<void> {
		finalizePendingDelete(false);
		const event = options.context.getCalendarEvents().find((candidate) => candidate.id === eventID);
		if (!event) return;
		options.context.invalidatePendingEventLoad();
		options.context.removeCalendarEvent(eventID);
		options.refreshEventCountAfterRender();
		pendingDelete = {
			event,
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
		const deleteAction = pendingDelete;
		clearPendingDelete(deleteAction, true);
		options.context.invalidatePendingEventLoad();
		if (!hasLocalEvent(eventID)) options.context.restoreCalendarEvent(deleteAction.event);
		options.refreshEventCountAfterRender();
	}

	function undoLastDelete(): boolean {
		if (!pendingDelete) return false;
		undoPendingDelete(pendingDelete.event.id);
		return true;
	}

	async function flushPendingDelete(): Promise<void> {
		const deleteAction = pendingDelete;
		finalizePendingDelete(true);
		if (!deleteAction) return;
		options.beginPersistence();
		try {
			await options.persistedEvents.deleteEvent(
				deleteAction.event.id,
				versionOfPendingDelete(deleteAction.event)
			);
			options.persistenceOrder.clearPersistedUpdatedAt(deleteAction.event.id);
		} catch (refusal) {
			await recoverFromFailedDelete(refusal, deleteAction.event);
		} finally {
			options.finishPersistence();
		}
	}

	function versionOfPendingDelete(event: DayTaskEvent): string | undefined {
		const rendered = typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined;
		return options.persistenceOrder.persistedUpdatedAt(event.id, rendered);
	}

	async function recoverFromFailedDelete(refusal: unknown, event: DayTaskEvent): Promise<void> {
		options.context.invalidatePendingEventLoad();
		if (isRefusalCode(refusal, calendarEventVersionConflictErrorCode)) {
			options.showPersistenceError(options.context.text.calendarDeleteVersionConflictError);
			await options.context.refreshCalendar();
			return;
		}
		options.showPersistenceError(options.context.text.deleteError);
		if (!hasLocalEvent(event.id)) options.context.restoreCalendarEvent(event);
		options.refreshEventCountAfterRender();
	}

	function finalizePendingDelete(shouldDismissToast: boolean): void {
		if (!pendingDelete) return;
		clearPendingDelete(pendingDelete, shouldDismissToast);
	}

	function clearPendingDelete(deleteAction: PendingCalendarDelete, shouldDismissToast: boolean): void {
		if (pendingDelete !== deleteAction) return;
		clearTimeout(deleteAction.timeoutID);
		pendingDelete = null;
		if (shouldDismissToast) options.dismissUndoToast();
	}

	function hasLocalEvent(eventID: string): boolean {
		return options.context.getCalendarEvents().some((event) => event.id === eventID);
	}

	return {
		deleteEvent,
		flushPendingDelete,
		undoLastDelete
	};
}
