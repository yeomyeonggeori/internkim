import type { Event as DayFlowEvent } from '@dayflow/core';
import type { CalendarDraftEventDOMActions } from './calendar-draft-event-dom';
import { CalendarDraftEventState } from './calendar-draft-events';
import type { CalendarEventActionsContext } from './calendar-event-actions';
import type { CalendarEvent } from './calendar-event-persistence';
import type { CalendarPersistedEventActions } from './calendar-persisted-event-actions';

type CalendarDraftEventPersistenceOptions = {
	context: CalendarEventActionsContext;
	draftEvents: CalendarDraftEventState;
	draftEventDOM: CalendarDraftEventDOMActions;
	persistedEvents: CalendarPersistedEventActions;
	refreshEventCountAfterRender: () => void;
	resetDraftEventTitle: (eventID: string) => Promise<void>;
	beginPersistence: () => void;
	finishPersistence: () => void;
	markEventPersisted: () => void;
	showPersistenceError: (error: unknown, fallback: string, versionConflictMessage?: string) => void;
};

export type CalendarDraftEventPersistenceActions = {
	deleteEvent: (eventID: string) => boolean;
	persistCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	saveCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	saveUpdatedEvent: (event: DayFlowEvent) => Promise<void>;
};

export function createCalendarDraftEventPersistenceActions(
	options: CalendarDraftEventPersistenceOptions
): CalendarDraftEventPersistenceActions {
	async function saveCreatedEvent(event: DayFlowEvent): Promise<void> {
		options.context.invalidatePendingEventLoad();
		options.draftEvents.addCreatedEvent(event);
		if (options.draftEvents.isPlaceholderTitle(event.title)) {
			void options.resetDraftEventTitle(event.id);
		}
		options.draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
		options.draftEventDOM.scheduleDraftEventVisibilitySync();
	}

	async function saveUpdatedEvent(event: DayFlowEvent): Promise<void> {
		if (!options.draftEvents.hasMeaningfulTitle(event)) {
			void options.resetDraftEventTitle(event.id);
			options.draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
			return;
		}
		options.draftEvents.retainDraftEvent(event);
		if (options.draftEvents.hasPendingCreate(event.id)) return;
		await persistCreatedEvent(event);
	}

	function deleteEvent(eventID: string): boolean {
		if (options.draftEvents.hasPendingCreate(eventID)) {
			options.context.invalidatePendingEventLoad();
			options.draftEvents.markDeletedDuringCreate(eventID);
			removeLocalEvent(eventID);
			return true;
		}
		if (!options.draftEvents.isDraftEvent(eventID)) return false;
		options.context.invalidatePendingEventLoad();
		options.draftEvents.removeDraftEvent(eventID);
		removeLocalEvent(eventID);
		return true;
	}

	function removeLocalEvent(eventID: string): void {
		options.context.removeCalendarEvent(eventID);
		options.refreshEventCountAfterRender();
	}

	async function persistCreatedEvent(event: DayFlowEvent): Promise<void> {
		await options.draftEvents.trackCreatedEvent(event, createEventOnServer);
	}

	async function createEventOnServer(event: DayFlowEvent): Promise<void> {
		if (options.draftEvents.isPlaceholderTitle(event.title)) return;
		options.beginPersistence();
		let persistedRevision = options.draftEvents.draftEventRevision(event.id);
		let savedEvent: CalendarEvent;
		let persistedUpdatedAt: string | undefined;
		let hasCreatedServerEvent = false;
		try {
			savedEvent = await options.persistedEvents.writeEvent('/calendar/api/events', 'POST', event);
			persistedUpdatedAt = savedEvent.updatedAt;
			hasCreatedServerEvent = true;
			while (!options.draftEvents.wasDeletedDuringCreate(event.id)) {
				const latestRevision = options.draftEvents.draftEventRevision(event.id);
				const latestEvent = options.draftEvents.draftEvent(event.id);
				if (!latestEvent || latestRevision === persistedRevision) break;
				persistedRevision = latestRevision;
				const expectedUpdatedAt = savedEvent.updatedAt;
				savedEvent = await options.persistedEvents.writeEvent(
					`/calendar/api/events/${encodeURIComponent(event.id)}`,
					'PUT',
					latestEvent,
					expectedUpdatedAt
				);
				persistedUpdatedAt = savedEvent.updatedAt;
			}
		} catch (error) {
			if (hasCreatedServerEvent && options.draftEvents.wasDeletedDuringCreate(event.id)) {
				options.draftEvents.removeDraftEvent(event.id);
				await deleteEventCreatedDuringPendingCreate(event.id, persistedUpdatedAt);
				return;
			}
			if (options.draftEvents.shouldReportCreateError(event.id)) {
				options.showPersistenceError(error, options.context.text.saveError);
			} else {
				options.draftEvents.removeDraftEvent(event.id);
			}
			options.finishPersistence();
			return;
		}
		if (options.draftEvents.wasDeletedDuringCreate(event.id)) {
			options.draftEvents.removeDraftEvent(event.id);
			await deleteEventCreatedDuringPendingCreate(event.id, savedEvent.updatedAt);
			return;
		}
		options.draftEvents.removeDraftEvent(event.id);
		options.draftEventDOM.scheduleDraftEventVisibilitySync();
		try {
			await options.persistedEvents.applyServerMetadata(event.id, savedEvent);
			if (!hasLocalEvent(event.id)) return;
			options.markEventPersisted();
		} catch (error) {
			if (!hasLocalEvent(event.id)) return;
			options.showPersistenceError(error, options.context.text.saveError);
			await options.context.refreshCalendar();
		} finally {
			options.finishPersistence();
		}
	}

	function hasLocalEvent(eventID: string): boolean {
		return options.context.getCalendarEvents().some((event) => event.id === eventID);
	}

	async function deleteEventCreatedDuringPendingCreate(
		eventID: string,
		expectedUpdatedAt: string | undefined
	): Promise<void> {
		try {
			await options.persistedEvents.deleteEvent(eventID, expectedUpdatedAt);
			options.context.invalidatePendingEventLoad();
			options.context.removeCalendarEvent(eventID);
			options.context.notifyEventsChanged();
			options.refreshEventCountAfterRender();
		} catch (error) {
			options.showPersistenceError(
				error,
				options.context.text.deleteError,
				options.context.text.calendarDeleteVersionConflictError
			);
			await options.context.refreshCalendar();
		} finally {
			options.finishPersistence();
		}
	}

	return {
		deleteEvent,
		persistCreatedEvent,
		saveCreatedEvent,
		saveUpdatedEvent
	};
}
