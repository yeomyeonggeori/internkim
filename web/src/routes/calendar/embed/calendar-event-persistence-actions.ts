import type { Event as DayFlowEvent } from '@dayflow/core';
import { toast } from 'svelte-sonner';
import { CalendarDraftEventState } from './calendar-draft-events';
import type { CalendarDraftEventDOMActions } from './calendar-draft-event-dom';
import { createCalendarDraftEventPersistenceActions } from './calendar-draft-event-persistence-actions';
import {
	createCalendarPersistedEventActions
} from './calendar-persisted-event-actions';
import {
	dismissCalendarDeleteUndoToast,
	showCalendarDeleteUndoToast
} from './calendar-delete-undo';
import type { CalendarEvent } from './calendar-event-persistence';
import { CalendarPersistenceError } from './calendar-event-persistence';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';
import type { CalendarEventActionsContext } from './calendar-event-actions';
import { CalendarEventPersistenceOrder } from './calendar-event-persistence-order';
import { createCalendarDeletePersistenceActions } from './calendar-delete-persistence-actions';

type CalendarEventPersistenceActionsContext = {
	context: CalendarEventActionsContext;
	draftEvents: CalendarDraftEventState;
	draftEventDOM: CalendarDraftEventDOMActions;
	programmaticUpdates: CalendarProgrammaticUpdateState;
	refreshEventCountAfterRender: () => void;
	refreshLocalEventSnapshot: () => void;
	resetDraftEventTitle: (eventID: string) => Promise<void>;
};

export type CalendarEventPersistenceActions = {
	saveCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	saveUpdatedEvent: (event: DayFlowEvent, previousEvent?: DayFlowEvent) => Promise<void>;
	deleteEvent: (eventID: string) => Promise<void>;
	flushPendingDelete: () => Promise<void>;
	persistCreatedEvent: (event: DayFlowEvent) => Promise<void>;
};

type CalendarEventPersistenceDependencies = {
	createPersistedEventActions: typeof createCalendarPersistedEventActions;
	dismissDeleteUndoToast: typeof dismissCalendarDeleteUndoToast;
	notifyError: (message: string) => void;
	showDeleteUndoToast: typeof showCalendarDeleteUndoToast;
};

export function createCalendarEventPersistenceActions(
	options: CalendarEventPersistenceActionsContext,
	dependencies: Partial<CalendarEventPersistenceDependencies> = {}
): CalendarEventPersistenceActions {
	const persistedEvents = (dependencies.createPersistedEventActions ?? createCalendarPersistedEventActions)(
		options.context,
		options.programmaticUpdates
	);
	const dismissDeleteUndoToast = dependencies.dismissDeleteUndoToast ?? dismissCalendarDeleteUndoToast;
	const notifyError = dependencies.notifyError ?? ((message: string) => toast.error(message));
	const showDeleteUndoToast = dependencies.showDeleteUndoToast ?? showCalendarDeleteUndoToast;
	const persistenceOrder = new CalendarEventPersistenceOrder();
	let activePersistenceCount = 0;
	const draftPersistence = createCalendarDraftEventPersistenceActions({
		context: options.context,
		draftEvents: options.draftEvents,
		draftEventDOM: options.draftEventDOM,
		persistedEvents,
		refreshEventCountAfterRender: options.refreshEventCountAfterRender,
		resetDraftEventTitle: options.resetDraftEventTitle,
		beginPersistence: beginEventPersistence,
		finishPersistence: finishEventPersistence,
		markEventPersisted,
		showPersistenceError: showEventPersistenceError
	});
	const deletePersistence = createCalendarDeletePersistenceActions({
		context: options.context,
		persistedEvents,
		persistenceOrder,
		refreshEventCountAfterRender: options.refreshEventCountAfterRender,
		beginPersistence: beginDeletePersistence,
		finishPersistence: finishEventPersistence,
		showPersistenceError: showEventPersistenceError,
		dismissUndoToast: dismissDeleteUndoToast,
		showUndoToast: showDeleteUndoToast
	});

	async function saveUpdatedEvent(event: DayFlowEvent, previousEvent?: DayFlowEvent): Promise<void> {
		if (options.programmaticUpdates.isActive(event.id)) return;
		options.context.invalidatePendingEventLoad();
		if (options.draftEvents.isDraftEvent(event.id)) {
			await draftPersistence.saveUpdatedEvent(event);
			return;
		}
		const revision = persistenceOrder.beginAction(event.id);
		await persistenceOrder.runLatestAction(event.id, revision, () =>
			persistUpdatedEvent(event, previousEvent, revision)
		);
	}

	async function deleteEvent(eventID: string): Promise<void> {
		const revision = persistenceOrder.beginAction(eventID);
		if (options.context.getSelectedAuditEventID() === eventID) {
			options.context.setSelectedAuditEventID(null);
		}
		if (draftPersistence.deleteEvent(eventID)) return;
		await deletePersistence.deleteEvent(eventID, revision);
	}

	async function persistUpdatedEvent(
		event: DayFlowEvent,
		previousEvent: DayFlowEvent | undefined,
		revision: number
	): Promise<void> {
		beginEventPersistence();
		let savedEvent: CalendarEvent;
		const visibleUpdatedAt = typeof event.meta?.updatedAt === 'string' ? event.meta.updatedAt : undefined;
		try {
			savedEvent = await persistedEvents.writeEvent(
				`/calendar/api/events/${encodeURIComponent(event.id)}`,
				'PUT',
				event,
				persistenceOrder.persistedUpdatedAt(event.id, visibleUpdatedAt),
				persistenceOrder.clientID,
				revision
			);
		} catch (error) {
			try {
				if (persistenceOrder.isLatestAction(event.id, revision)) {
					showEventPersistenceError(error, options.context.text.saveError);
					if (isCalendarEventVersionConflict(error)) {
						persistenceOrder.clearPersistedUpdatedAt(event.id);
						await options.context.refreshCalendar();
					} else {
						await rollbackUpdatedEvent(previousEvent);
					}
				}
			} finally {
				finishEventPersistence();
			}
			return;
		}
		persistenceOrder.recordPersistedUpdatedAt(event.id, savedEvent.updatedAt);
		if (!persistenceOrder.isLatestAction(event.id, revision)) {
			finishEventPersistence();
			return;
		}
		try {
			await persistedEvents.applyServerMetadata(event.id, savedEvent);
			if (persistenceOrder.isLatestAction(event.id, revision)) {
				persistenceOrder.clearPersistedUpdatedAt(event.id, savedEvent.updatedAt);
			}
			markEventPersisted();
		} catch (error) {
			if (!persistenceOrder.isLatestAction(event.id, revision)) return;
			showEventPersistenceError(error, options.context.text.saveError);
			await options.context.refreshCalendar();
			if (persistenceOrder.isLatestAction(event.id, revision)) {
				persistenceOrder.clearPersistedUpdatedAt(event.id, savedEvent.updatedAt);
			}
		} finally {
			finishEventPersistence();
		}
	}

	async function rollbackUpdatedEvent(previousEvent?: DayFlowEvent): Promise<void> {
		if (!previousEvent) {
			await options.context.refreshCalendar();
			return;
		}
		options.context.restoreCalendarEvent(previousEvent);
		options.refreshLocalEventSnapshot();
	}

	function beginEventPersistence(): void {
		activePersistenceCount += 1;
		if (activePersistenceCount === 1) options.context.setIsSaving(true);
		options.context.setStatusMessage('');
		options.context.setErrorMessage('');
	}

	function beginDeletePersistence(): void {
		activePersistenceCount += 1;
		if (activePersistenceCount === 1) options.context.setIsSaving(true);
		options.context.setErrorMessage('');
	}

	function finishEventPersistence(): void {
		activePersistenceCount = Math.max(0, activePersistenceCount - 1);
		if (activePersistenceCount === 0) options.context.setIsSaving(false);
	}

	function markEventPersisted(): void {
		options.context.setStatusMessage(options.context.text.shared);
		options.refreshLocalEventSnapshot();
		options.context.notifyEventsChanged();
	}

	function showEventPersistenceError(
		error: unknown,
		fallback: string,
		versionConflictMessage = options.context.text.calendarEventVersionConflictError
	): void {
		notifyError(eventPersistenceErrorMessage(error, fallback, versionConflictMessage));
	}

	function eventPersistenceErrorMessage(
		error: unknown,
		fallback: string,
		versionConflictMessage: string
	): string {
		if (!(error instanceof CalendarPersistenceError)) return fallback;
		if (error.code === 'calendar_target_unavailable') {
			return options.context.text.calendarTargetUnavailableError;
		}
		if (error.code === 'calendar_event_version_conflict') {
			return versionConflictMessage;
		}
		return error.message;
	}

	function isCalendarEventVersionConflict(error: unknown): boolean {
		return error instanceof CalendarPersistenceError && error.code === 'calendar_event_version_conflict';
	}

	return {
		saveCreatedEvent: draftPersistence.saveCreatedEvent,
		saveUpdatedEvent,
		deleteEvent,
		flushPendingDelete: deletePersistence.flushPendingDelete,
		persistCreatedEvent: draftPersistence.persistCreatedEvent
	};
}
