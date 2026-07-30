import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarEventActionsContext } from './calendar-event-actions';
import {
	calendarDeleteUndoTimeoutMs,
	type showCalendarDeleteUndoToast
} from './calendar-delete-undo';
import {
	CalendarPersistenceError,
	calendarEventVersionConflictErrorCode,
	isCalendarPersistenceErrorCode
} from './calendar-event-persistence';
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
	waitForCancellationRetry?: (delay: number) => Promise<void>;
};

export type CalendarDeletePersistenceActions = {
	deleteEvent: (eventID: string, revision: number) => Promise<void>;
	flushPendingDelete: () => Promise<void>;
	undoLastDelete: () => boolean;
};

type CalendarDeleteIntentResult =
	| { isSuccessful: true }
	| { isSuccessful: false; error: unknown };

const calendarDeleteIntentCancellationRetryDelaysMs = [100, 200] as const;

type PendingCalendarDelete = {
	event: DayFlowEvent;
	operationID: string;
	timeoutID: ReturnType<typeof setTimeout>;
};

export function createCalendarDeletePersistenceActions(
	options: CalendarDeletePersistenceOptions
): CalendarDeletePersistenceActions {
	let pendingDelete: PendingCalendarDelete | null = null;

	async function deleteEvent(eventID: string, revision: number): Promise<void> {
		finalizePendingDelete(false);
		const event = options.context.getCalendarEvents().find((candidate) => candidate.id === eventID);
		if (!event) return;
		const expectedUpdatedAt = deleteExpectedUpdatedAt(event);
		const operationID = globalThis.crypto.randomUUID();
		const registrationResult = settleDeleteIntent(
			options.persistedEvents.createDeleteIntent(
				eventID,
				operationID,
				options.persistenceOrder.clientID,
				revision,
				expectedUpdatedAt
			)
		);
		options.context.invalidatePendingEventLoad();
		options.context.removeCalendarEvent(eventID);
		options.refreshEventCountAfterRender();
		const deleteAction: PendingCalendarDelete = {
			event,
			operationID,
			timeoutID: setTimeout(() => {
				void flushPendingDelete();
			}, calendarDeleteUndoTimeoutMs)
		};
		pendingDelete = deleteAction;
		options.showUndoToast({
			message: options.context.text.deleteUndoMessage,
			actionLabel: options.context.text.deleteUndoAction,
			undo: () => undoPendingDelete(eventID)
		});
		void registrationResult.then(async (result) => {
			const cancellationResult = await cancelAmbiguousDeleteIntentRegistration(
				eventID,
				operationID,
				revision,
				result
			);
			await options.persistenceOrder.runLatestAction(eventID, revision, () =>
				handleDeleteIntentRegistration(deleteAction, result, cancellationResult)
			);
		});
	}

	async function cancelAmbiguousDeleteIntentRegistration(
		eventID: string,
		operationID: string,
		revision: number,
		registrationResult: CalendarDeleteIntentResult
	): Promise<CalendarDeleteIntentResult> {
		if (registrationResult.isSuccessful || !isAmbiguousRegistrationFailure(registrationResult.error)) {
			return { isSuccessful: true };
		}
		return cancelDeleteIntentWithRetry(eventID, operationID, revision + 1);
	}

	async function cancelDeleteIntentWithRetry(
		eventID: string,
		operationID: string,
		sequence: number
	): Promise<CalendarDeleteIntentResult> {
		let cancellationResult = await cancelDeleteIntent(eventID, operationID, sequence);
		for (const retryDelay of calendarDeleteIntentCancellationRetryDelaysMs) {
			if (cancellationResult.isSuccessful || !isRetryableCancellationFailure(cancellationResult.error)) {
				return cancellationResult;
			}
			await (options.waitForCancellationRetry ?? waitForDeleteIntentCancellationRetry)(retryDelay);
			cancellationResult = await cancelDeleteIntent(eventID, operationID, sequence);
		}
		return cancellationResult;
	}

	function cancelDeleteIntent(
		eventID: string,
		operationID: string,
		sequence: number
	): Promise<CalendarDeleteIntentResult> {
		return settleDeleteIntent(
			options.persistedEvents.cancelDeleteIntent(
				eventID,
				operationID,
				options.persistenceOrder.clientID,
				sequence
			)
		);
	}

	function undoPendingDelete(eventID: string): void {
		if (!pendingDelete || pendingDelete.event.id !== eventID) return;
		const deleteAction = pendingDelete;
		clearPendingDelete(deleteAction, true);
		const revision = options.persistenceOrder.beginAction(eventID);
		const cancellationResult = cancelDeleteIntentWithRetry(
			eventID,
			deleteAction.operationID,
			revision
		);
		options.context.invalidatePendingEventLoad();
		if (!hasLocalEvent(eventID)) {
			options.context.restoreCalendarEvent(deleteAction.event);
		}
		options.refreshEventCountAfterRender();
		void cancellationResult.then((result) =>
			options.persistenceOrder.runLatestAction(eventID, revision, () =>
				handleDeleteIntentCancellation(eventID, revision, result)
			)
		);
	}

	function undoLastDelete(): boolean {
		if (!pendingDelete) return false;
		undoPendingDelete(pendingDelete.event.id);
		return true;
	}

	function flushPendingDelete(): Promise<void> {
		finalizePendingDelete(true);
		return Promise.resolve();
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

	function deleteExpectedUpdatedAt(event: DayFlowEvent): string {
		const visibleUpdatedAt = typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined;
		return options.persistenceOrder.persistedUpdatedAt(
			event.id,
			visibleUpdatedAt
		) ?? '';
	}

	async function handleDeleteIntentRegistration(
		deleteAction: PendingCalendarDelete,
		result: CalendarDeleteIntentResult,
		cancellationResult: CalendarDeleteIntentResult
	): Promise<void> {
		if (result.isSuccessful) {
			options.persistenceOrder.clearPersistedUpdatedAt(deleteAction.event.id);
			options.context.invalidatePendingEventLoad();
			options.context.removeCalendarEvent(deleteAction.event.id);
			options.refreshEventCountAfterRender();
			return;
		}
		clearPendingDelete(deleteAction, true);
		options.beginPersistence();
		try {
			const isVersionConflict = isCalendarEventVersionConflict(result.error);
			const isCancellationUnconfirmed = !cancellationResult.isSuccessful;
			if (!isVersionConflict && !isCancellationUnconfirmed && !hasLocalEvent(deleteAction.event.id)) {
				options.context.restoreCalendarEvent(deleteAction.event);
			}
			if (isVersionConflict) {
				options.persistenceOrder.clearPersistedUpdatedAt(deleteAction.event.id);
			}
			showDeletePersistenceError(result.error);
			await options.context.refreshCalendar();
			if (isCancellationUnconfirmed) {
				options.context.removeCalendarEvent(deleteAction.event.id);
				options.refreshEventCountAfterRender();
			} else {
				hidePendingDeleteAfterRefresh();
			}
		} finally {
			options.finishPersistence();
		}
	}

	async function handleDeleteIntentCancellation(
		eventID: string,
		revision: number,
		result: CalendarDeleteIntentResult
	): Promise<void> {
		options.beginPersistence();
		try {
			if (!result.isSuccessful) showDeletePersistenceError(result.error);
			await options.context.refreshCalendar();
			if (options.persistenceOrder.isLatestAction(eventID, revision)) {
				options.persistenceOrder.clearPersistedUpdatedAt(eventID);
			}
			if (result.isSuccessful) {
				hidePendingDeleteAfterRefresh();
			} else {
				options.context.removeCalendarEvent(eventID);
				options.refreshEventCountAfterRender();
			}
		} finally {
			options.finishPersistence();
		}
	}

	function showDeletePersistenceError(error: unknown): void {
		options.showPersistenceError(
			error,
			options.context.text.deleteError,
			options.context.text.calendarDeleteVersionConflictError
		);
	}

	function isCalendarEventVersionConflict(error: unknown): boolean {
		return isCalendarPersistenceErrorCode(error, calendarEventVersionConflictErrorCode);
	}

	function isAmbiguousRegistrationFailure(error: unknown): boolean {
		return !(error instanceof CalendarPersistenceError) || error.code === 'unknown';
	}

	function isRetryableCancellationFailure(error: unknown): boolean {
		if (!(error instanceof CalendarPersistenceError)) return true;
		return error.code === 'unknown' || error.code === 'calendar_target_unavailable';
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
		undoLastDelete
	};
}

function waitForDeleteIntentCancellationRetry(delay: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, delay));
}

function settleDeleteIntent(operation: Promise<unknown>): Promise<CalendarDeleteIntentResult> {
	return operation.then(
		() => ({ isSuccessful: true }),
		(error: unknown) => ({ isSuccessful: false, error })
	);
}
