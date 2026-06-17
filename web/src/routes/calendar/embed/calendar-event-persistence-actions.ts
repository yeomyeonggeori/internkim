import type { Event as DayFlowEvent } from '@dayflow/core';
import { CalendarDraftEventState } from './calendar-draft-events';
import type { CalendarDraftEventDOMActions } from './calendar-draft-event-dom';
import {
	createCalendarPersistedEventActions
} from './calendar-persisted-event-actions';
import type { CalendarEvent } from './calendar-event-persistence';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';
import type { CalendarEventActionsContext } from './calendar-event-actions';

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
	saveUpdatedEvent: (event: DayFlowEvent) => Promise<void>;
	deleteEvent: (eventID: string) => Promise<void>;
	persistCreatedEvent: (event: DayFlowEvent) => Promise<void>;
};

export function createCalendarEventPersistenceActions(
	options: CalendarEventPersistenceActionsContext
): CalendarEventPersistenceActions {
	const persistedEvents = createCalendarPersistedEventActions(options.context, options.programmaticUpdates);

	async function saveCreatedEvent(event: DayFlowEvent): Promise<void> {
		options.draftEvents.addCreatedEvent(event);
		if (options.draftEvents.isPlaceholderTitle(event.title)) {
			void options.resetDraftEventTitle(event.id);
		}
		options.draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
		options.draftEventDOM.scheduleDraftEventVisibilitySync();
	}

	async function saveUpdatedEvent(event: DayFlowEvent): Promise<void> {
		if (options.programmaticUpdates.isActive(event.id)) return;
		if (options.draftEvents.isDraftEvent(event.id)) {
			await saveUpdatedDraftEvent(event);
			return;
		}
		await persistEvent(`/calendar/api/events/${encodeURIComponent(event.id)}`, 'PUT', event);
	}

	async function saveUpdatedDraftEvent(event: DayFlowEvent): Promise<void> {
		if (!options.draftEvents.hasMeaningfulTitle(event)) {
			void options.resetDraftEventTitle(event.id);
			options.draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
			return;
		}
		options.draftEvents.removeDraftEvent(event.id);
		await persistCreatedEvent(event);
	}

	async function deleteEvent(eventID: string): Promise<void> {
		if (options.context.getSelectedAuditEventID() === eventID) {
			options.context.setSelectedAuditEventID(null);
		}
		if (deleteDraftEvent(eventID)) return;
		if (deletePendingCreateEvent(eventID)) return;
		await deletePersistedEvent(eventID);
	}

	function deleteDraftEvent(eventID: string): boolean {
		if (!options.draftEvents.isDraftEvent(eventID)) return false;
		options.draftEvents.removeDraftEvent(eventID);
		options.context.removeCalendarEvent(eventID);
		options.refreshEventCountAfterRender();
		return true;
	}

	function deletePendingCreateEvent(eventID: string): boolean {
		if (!options.draftEvents.hasPendingCreate(eventID)) return false;
		options.draftEvents.markDeletedDuringCreate(eventID);
		options.context.removeCalendarEvent(eventID);
		options.refreshEventCountAfterRender();
		return true;
	}

	async function deletePersistedEvent(eventID: string): Promise<void> {
		beginDeletePersistence();
		try {
			await persistedEvents.deleteEvent(eventID);
			options.context.removeCalendarEvent(eventID);
			options.context.notifyEventsChanged();
			options.refreshEventCountAfterRender();
		} catch (error) {
			showEventPersistenceError(error, options.context.text.deleteError);
			await options.context.refreshCalendar();
		} finally {
			finishEventPersistence();
		}
	}

	async function createEventOnServer(event: DayFlowEvent): Promise<void> {
		if (options.draftEvents.isPlaceholderTitle(event.title)) return;
		beginEventPersistence();
		let savedEvent: CalendarEvent;
		try {
			savedEvent = await persistedEvents.writeEvent('/calendar/api/events', 'POST', event);
		} catch (error) {
			if (options.draftEvents.shouldReportCreateError(event.id)) {
				showEventPersistenceError(error, options.context.text.saveError);
			}
			finishEventPersistence();
			return;
		}
		if (options.draftEvents.wasDeletedDuringCreate(event.id)) {
			await deleteEventCreatedDuringPendingCreate(event.id);
			return;
		}
		try {
			await persistedEvents.applyServerMetadata(event.id, savedEvent);
			markEventPersisted();
		} catch (error) {
			showEventPersistenceError(error, options.context.text.saveError);
		} finally {
			finishEventPersistence();
		}
	}

	async function deleteEventCreatedDuringPendingCreate(eventID: string): Promise<void> {
		try {
			await persistedEvents.deleteEvent(eventID);
			options.context.notifyEventsChanged();
			options.refreshEventCountAfterRender();
		} catch (error) {
			showEventPersistenceError(error, options.context.text.deleteError);
			await options.context.refreshCalendar();
		} finally {
			finishEventPersistence();
		}
	}

	async function persistEvent(path: string, method: 'POST' | 'PUT', event: DayFlowEvent): Promise<void> {
		beginEventPersistence();
		try {
			const savedEvent = await persistedEvents.writeEvent(path, method, event);
			await persistedEvents.applyServerMetadata(event.id, savedEvent);
			markEventPersisted();
		} catch (error) {
			showEventPersistenceError(error, options.context.text.saveError);
		} finally {
			finishEventPersistence();
		}
	}

	async function persistCreatedEvent(event: DayFlowEvent): Promise<void> {
		await options.draftEvents.trackCreatedEvent(event, createEventOnServer);
	}

	function beginEventPersistence(): void {
		options.context.setIsSaving(true);
		options.context.setStatusMessage('');
		options.context.setErrorMessage('');
	}

	function beginDeletePersistence(): void {
		options.context.setIsSaving(true);
		options.context.setErrorMessage('');
	}

	function finishEventPersistence(): void {
		options.context.setIsSaving(false);
	}

	function markEventPersisted(): void {
		options.context.setStatusMessage(options.context.text.shared);
		options.refreshLocalEventSnapshot();
		options.context.notifyEventsChanged();
	}

	function showEventPersistenceError(error: unknown, fallback: string): void {
		options.context.setErrorMessage(error instanceof Error ? error.message : fallback);
	}

	return {
		saveCreatedEvent,
		saveUpdatedEvent,
		deleteEvent,
		persistCreatedEvent
	};
}
