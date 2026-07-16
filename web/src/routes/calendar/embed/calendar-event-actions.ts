import type { Event as DayFlowEvent } from '@dayflow/core';
import { CalendarDraftEventState, type DraftEventParams } from './calendar-draft-events';
import {
	createCalendarDraftEventDOMActions,
	type CalendarDraftEventDOMActions
} from './calendar-draft-event-dom';
import { createCalendarEventDraftActions } from './calendar-event-draft-actions';
import {
	createCalendarEventPersistenceActions,
	type CalendarEventPersistenceActions
} from './calendar-event-persistence-actions';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';

export type CalendarEventActionsContext = {
	isBrowser: () => boolean;
	getCurrentDate: () => Date | null | undefined;
	getStageElement: () => HTMLElement | null;
	getSelectedAuditEventID: () => string | null;
	setSelectedAuditEventID: (eventID: string | null) => void;
	getCalendarEvents: () => DayFlowEvent[];
	addCalendarEvent: (event: DayFlowEvent) => void;
	restoreCalendarEvent: (event: DayFlowEvent) => void;
	removeCalendarEvent: (eventID: string) => void;
	updateCalendarEvent: (eventID: string, changes: Partial<DayFlowEvent>, shouldRender: boolean) => Promise<void>;
	setEventCount: (eventCount: number) => void;
	setVisibleEvents: (events: DayFlowEvent[]) => void;
	setIsSaving: (isSaving: boolean) => void;
	setStatusMessage: (message: string) => void;
	setErrorMessage: (message: string) => void;
	openEventDetails: (eventID: string) => void;
	openMobileEventEditor: (event: DayFlowEvent) => void;
	notifyEventsChanged: () => void;
	refreshCalendar: () => Promise<void>;
	invalidatePendingEventLoad: () => void;
	text: {
		calendarDeleteVersionConflictError: string;
		calendarEventVersionConflictError: string;
		calendarTargetUnavailableError: string;
		deleteError: string;
		deleteUndoAction: string;
		deleteUndoMessage: string;
		draftTitlePlaceholder: string;
		saveError: string;
		shared: string;
	};
};

export type CalendarEventActions = {
	createAllDaySingleEvent: (dateKey: string) => DayFlowEvent | null;
	createQuickEvent: () => DayFlowEvent;
	openAllDaySingleEventMobileEditor: (dateKey: string) => void;
	openQuickEventMobileEditor: () => void;
	openTimelineSingleEventMobileEditor: (startDate: Date) => void;
	openEventMobileEditor: (eventID: string) => void;
	createMonthRangeEvent: (selection: MonthRangeSelection) => DayFlowEvent;
	createMonthSingleDayEvent: (dateKey: string) => DayFlowEvent | null;
	createTimelineSingleEvent: (startDate: Date) => DayFlowEvent | null;
	createTimelineRangeEvent: (firstDate: Date, secondDate: Date) => DayFlowEvent | null;
	saveCreatedEvent: (event: DayFlowEvent) => Promise<void>;
	saveUpdatedEvent: (event: DayFlowEvent, previousEvent?: DayFlowEvent) => Promise<void>;
	deleteEvent: (eventID: string) => Promise<void>;
	flushPendingDelete: () => Promise<void>;
	flushPendingDeleteOnPageHide: () => void;
	scheduleDraftTitleInputPlaceholderUpdates: () => void;
	scheduleDraftEventVisibilitySync: () => void;
};

export function createCalendarEventActions(
	context: CalendarEventActionsContext,
	draftEvents: CalendarDraftEventState,
	programmaticUpdates: CalendarProgrammaticUpdateState
): CalendarEventActions {
	let persistenceActions: CalendarEventPersistenceActions;
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
	persistenceActions = createCalendarEventPersistenceActions({
		context,
		draftEvents,
		draftEventDOM,
		programmaticUpdates,
		refreshEventCountAfterRender,
		refreshLocalEventSnapshot,
		resetDraftEventTitle
	});

	function addDraftEvent(params: DraftEventParams): DayFlowEvent {
		context.invalidatePendingEventLoad();
		const event = draftEvents.createDraftEvent(params);
		draftEvents.addCreatedEvent(event);
		context.addCalendarEvent(event);
		refreshLocalEventSnapshotWithEvent(event);
		return event;
	}
	const draftAction = createCalendarEventDraftActions({
		addDraftEvent,
		getCurrentDate: context.getCurrentDate
	});

	function openQuickEventMobileEditor(): void {
		context.openMobileEventEditor(draftAction.createQuickEvent());
	}

	function openTimelineSingleEventMobileEditor(startDate: Date): void {
		const event = draftAction.createTimelineSingleEvent(startDate);
		if (!event) return;
		context.openMobileEventEditor(event);
	}

	function openAllDaySingleEventMobileEditor(dateKey: string): void {
		const event = draftAction.createAllDaySingleEvent(dateKey);
		if (!event) return;
		context.openMobileEventEditor(event);
	}

	function openEventMobileEditor(eventID: string): void {
		const event = context.getCalendarEvents().find((calendarEvent) => calendarEvent.id === eventID);
		if (!event) return;
		context.openMobileEventEditor(event);
	}

	async function resetDraftEventTitle(eventID: string): Promise<void> {
		await programmaticUpdates.run(eventID, () => context.updateCalendarEvent(eventID, { title: '' }, false));
		context.setVisibleEvents(context.getCalendarEvents());
		draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates();
		draftEventDOM.scheduleDraftEventVisibilitySync();
	}

	async function persistCreatedEvent(event: DayFlowEvent): Promise<void> {
		await persistenceActions.persistCreatedEvent(event);
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

	function refreshLocalEventSnapshotWithEvent(event: DayFlowEvent): void {
		const events = context.getCalendarEvents().filter((calendarEvent) => calendarEvent.id !== event.id);
		const nextEvents = [...events, event];
		context.setEventCount(nextEvents.length);
		context.setVisibleEvents(nextEvents);
	}

	return {
		createQuickEvent: draftAction.createQuickEvent,
		createAllDaySingleEvent: draftAction.createAllDaySingleEvent,
		openAllDaySingleEventMobileEditor,
		openQuickEventMobileEditor,
		openTimelineSingleEventMobileEditor,
		openEventMobileEditor,
		createMonthRangeEvent: draftAction.createMonthRangeEvent,
		createMonthSingleDayEvent: draftAction.createMonthSingleDayEvent,
		createTimelineSingleEvent: draftAction.createTimelineSingleEvent,
		createTimelineRangeEvent: draftAction.createTimelineRangeEvent,
		saveCreatedEvent: persistenceActions.saveCreatedEvent,
		saveUpdatedEvent: persistenceActions.saveUpdatedEvent,
		deleteEvent: persistenceActions.deleteEvent,
		flushPendingDelete: persistenceActions.flushPendingDelete,
		flushPendingDeleteOnPageHide: persistenceActions.flushPendingDeleteOnPageHide,
		scheduleDraftTitleInputPlaceholderUpdates: draftEventDOM.scheduleDraftTitleInputPlaceholderUpdates,
		scheduleDraftEventVisibilitySync: draftEventDOM.scheduleDraftEventVisibilitySync
	};
}
