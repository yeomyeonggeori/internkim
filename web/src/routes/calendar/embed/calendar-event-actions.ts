import type { Event as DayFlowEvent } from '@dayflow/core';
import { CalendarDraftEventState, type DraftEventParams } from './calendar-draft-events';
import {
	monthRangeDraftEventParams,
	monthSingleDayDraftEventParams,
	quickDraftEventParams
} from './calendar-draft-event-params';
import {
	createCalendarDraftEventDOMActions,
	type CalendarDraftEventDOMActions
} from './calendar-draft-event-dom';
import {
	createCalendarPersistedEventActions
} from './calendar-persisted-event-actions';
import type { CalendarEvent } from './calendar-event-persistence';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';

type CalendarEventActionsContext = {
	isBrowser: () => boolean;
	getCurrentDate: () => Date | null | undefined;
	getStageElement: () => HTMLElement | null;
	getSelectedAuditEventID: () => string | null;
	setSelectedAuditEventID: (eventID: string | null) => void;
	getCalendarEvents: () => DayFlowEvent[];
	addCalendarEvent: (event: DayFlowEvent) => void;
	updateCalendarEvent: (eventID: string, changes: Partial<DayFlowEvent>, shouldRender: boolean) => Promise<void>;
	setEventCount: (eventCount: number) => void;
	setVisibleEvents: (events: DayFlowEvent[]) => void;
	setIsSaving: (isSaving: boolean) => void;
	setStatusMessage: (message: string) => void;
	setErrorMessage: (message: string) => void;
	openEventDetails: (eventID: string) => void;
	refreshCalendar: () => Promise<void>;
	text: {
		deleteError: string;
		draftTitlePlaceholder: string;
		saveError: string;
		shared: string;
	};
};

export type CalendarEventActions = {
	createQuickEvent: () => void;
	createMonthRangeEvent: (selection: MonthRangeSelection) => void;
	createMonthSingleDayEvent: (dateKey: string) => void;
	saveCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	saveUpdatedEvent: (event: DayFlowEvent) => Promise<void>;
	deleteEvent: (eventID: string) => Promise<void>;
	scheduleDraftTitleInputPlaceholderUpdates: () => void;
	scheduleDraftEventVisibilitySync: () => void;
};

export function createCalendarEventActions(
	context: CalendarEventActionsContext,
	draftEvents: CalendarDraftEventState,
	programmaticUpdates: CalendarProgrammaticUpdateState
): CalendarEventActions {
	let lastMonthCellCreationTime = 0;
	const persistedEvents = createCalendarPersistedEventActions(context, programmaticUpdates);
	const draftEventDOM: CalendarDraftEventDOMActions = createCalendarDraftEventDOMActions(
		{
			isBrowser: context.isBrowser,
			getStageElement: context.getStageElement,
			getCalendarEvents: context.getCalendarEvents,
			updateCalendarEvent: context.updateCalendarEvent,
			openEventDetails: context.openEventDetails,
			resetDraftEventTitle,
			persistCreatedEvent,
			text: {
				get draftTitlePlaceholder() {
					return context.text.draftTitlePlaceholder;
				}
			}
		},
		draftEvents,
		programmaticUpdates
	);

	function addDraftEvent(params: DraftEventParams): DayFlowEvent {
		const event = draftEvents.createDraftEvent(params);
		context.addCalendarEvent(event);
		refreshLocalEventSnapshot();
		draftEventDOM.openEventDetailsAfterRender(event.id);
		return event;
	}

	function createQuickEvent(): void {
		const baseDate = context.getCurrentDate() ?? new Date();
		addDraftEvent(quickDraftEventParams(baseDate));
	}

	function createMonthRangeEvent(selection: MonthRangeSelection): void {
		const params = monthRangeDraftEventParams(selection);
		if (!params) {
			createMonthSingleDayEvent(selection.startDateKey);
			return;
		}
		addDraftEvent(params);
	}

	function createMonthSingleDayEvent(dateKey: string): void {
		if (Date.now() - lastMonthCellCreationTime < 250) return;
		lastMonthCellCreationTime = Date.now();
		addDraftEvent(monthSingleDayDraftEventParams(dateKey));
	}

	async function saveCreatedEvent(event: DayFlowEvent): Promise<void> {
		draftEvents.addCreatedEvent(event);
		if (draftEvents.isPlaceholderTitle(event.title)) {
			void resetDraftEventTitle(event.id);
		}
		draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
		draftEventDOM.scheduleDraftEventVisibilitySync();
	}

	async function saveUpdatedEvent(event: DayFlowEvent): Promise<void> {
		if (programmaticUpdates.isActive(event.id)) return;
		if (draftEvents.isDraftEvent(event.id)) {
			if (!draftEvents.hasMeaningfulTitle(event)) {
				void resetDraftEventTitle(event.id);
				draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
				return;
			}
			draftEvents.removeDraftEvent(event.id);
			await persistCreatedEvent(event);
			return;
		}
		await persistEvent(`/calendar/api/events/${encodeURIComponent(event.id)}`, 'PUT', event);
	}

	async function deleteEvent(eventID: string): Promise<void> {
		if (context.getSelectedAuditEventID() === eventID) {
			context.setSelectedAuditEventID(null);
		}
		if (draftEvents.isDraftEvent(eventID)) {
			draftEvents.removeDraftEvent(eventID);
			refreshEventCountAfterRender();
			return;
		}
		if (draftEvents.hasPendingCreate(eventID)) {
			draftEvents.markDeletedDuringCreate(eventID);
			refreshEventCountAfterRender();
			return;
		}
		beginDeletePersistence();
		try {
			await persistedEvents.deleteEvent(eventID);
			refreshEventCountAfterRender();
		} catch (error) {
			showEventPersistenceError(error, context.text.deleteError);
			await context.refreshCalendar();
		} finally {
			finishEventPersistence();
		}
	}

	async function createEventOnServer(event: DayFlowEvent): Promise<void> {
		if (draftEvents.isPlaceholderTitle(event.title)) {
			return;
		}
		beginEventPersistence();
		let savedEvent: CalendarEvent;
		try {
			savedEvent = await persistedEvents.writeEvent('/calendar/api/events', 'POST', event);
		} catch (error) {
			if (draftEvents.shouldReportCreateError(event.id)) {
				showEventPersistenceError(error, context.text.saveError);
			}
			finishEventPersistence();
			return;
		}
		if (draftEvents.wasDeletedDuringCreate(event.id)) {
			await deleteEventCreatedDuringPendingCreate(event.id);
			return;
		}
		try {
			await persistedEvents.applyServerMetadata(event.id, savedEvent);
			markEventPersisted();
		} catch (error) {
			showEventPersistenceError(error, context.text.saveError);
		} finally {
			finishEventPersistence();
		}
	}

	async function deleteEventCreatedDuringPendingCreate(eventID: string): Promise<void> {
		try {
			await persistedEvents.deleteEvent(eventID);
			refreshEventCountAfterRender();
		} catch (error) {
			showEventPersistenceError(error, context.text.deleteError);
			await context.refreshCalendar();
		} finally {
			finishEventPersistence();
		}
	}

	async function resetDraftEventTitle(eventID: string): Promise<void> {
		await programmaticUpdates.run(eventID, () => context.updateCalendarEvent(eventID, { title: '' }, false));
		context.setVisibleEvents(context.getCalendarEvents());
		draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
		draftEventDOM.scheduleDraftEventVisibilitySync();
	}

	async function persistEvent(path: string, method: 'POST' | 'PUT', event: DayFlowEvent): Promise<void> {
		beginEventPersistence();
		try {
			const savedEvent = await persistedEvents.writeEvent(path, method, event);
			await persistedEvents.applyServerMetadata(event.id, savedEvent);
			markEventPersisted();
		} catch (error) {
			showEventPersistenceError(error, context.text.saveError);
		} finally {
			finishEventPersistence();
		}
	}

	async function persistCreatedEvent(event: DayFlowEvent): Promise<void> {
		await draftEvents.trackCreatedEvent(event, createEventOnServer);
	}

	function beginEventPersistence(): void {
		context.setIsSaving(true);
		context.setStatusMessage('');
		context.setErrorMessage('');
	}

	function beginDeletePersistence(): void {
		context.setIsSaving(true);
		context.setErrorMessage('');
	}

	function finishEventPersistence(): void {
		context.setIsSaving(false);
	}

	function markEventPersisted(): void {
		context.setStatusMessage(context.text.shared);
		refreshLocalEventSnapshot();
	}

	function refreshEventCountAfterRender(): void {
		if (!context.isBrowser()) return;
		requestAnimationFrame(refreshLocalEventSnapshot);
	}

	function refreshLocalEventSnapshot(): void {
		const events = context.getCalendarEvents();
		context.setEventCount(events.length);
		context.setVisibleEvents(events);
	}

	function showEventPersistenceError(error: unknown, fallback: string): void {
		context.setErrorMessage(error instanceof Error ? error.message : fallback);
	}

	return {
		createQuickEvent,
		createMonthRangeEvent,
		createMonthSingleDayEvent,
		saveCreatedEvent,
		saveUpdatedEvent,
		deleteEvent,
		scheduleDraftTitleInputPlaceholderUpdates: draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates,
		scheduleDraftEventVisibilitySync: draftEventDOM.scheduleDraftEventVisibilitySync
	};
}
