import type { Event as DayFlowEvent, Locale } from '@dayflow/core';
import { useCalendarApp, type ViewType } from '@dayflow/svelte';
import type { CalendarLocaleText } from '../text';
import {
	broadcastCalendarEventsChanged,
	broadcastCalendarView
} from '../refresh-signal.svelte';
import { createCalendarConflictActions } from './calendar-conflict-actions';
import {
	createCalendarDraftPopoverActions,
	type CalendarDraftPopoverActions
} from './calendar-draft-popover-actions';
import { CalendarDraftEventState } from './calendar-draft-events';
import {
	createCalendarEventActions,
	type CalendarEventActions
} from './calendar-event-actions';
import {
	createCalendarEventLoader,
	type CalendarEventLoader
} from './calendar-event-loader';
import { createCalendarSelectedMonthDateActions } from './calendar-month-selection';
import { CalendarProgrammaticUpdateState } from './calendar-programmatic-updates';
import { shouldPreserveLocalCalendarEvent } from './calendar-visible-events';
import { createCalendarPageAppOptions } from './calendar-page-app-options';
import { createCalendarPageEventDetails } from './calendar-page-event-details';
import { createCalendarPageEventSelection } from './calendar-page-event-selection';
import { createCalendarPageInteractionServices } from './calendar-page-interaction-services';
import { createCalendarPageRenderSync } from './calendar-page-render-sync';
import type { CalendarEmbedPageState } from './calendar-page-state.svelte';

type CalendarPageControllerContext = {
	isBrowser: () => boolean;
	getCalendarLocale: () => Locale;
	getIsMobileTwoDayWeekView: () => boolean;
	getLocaleCode: () => string;
	initialCalendarDate: () => Date;
	initialCalendarView: () => ViewType;
	setVisibleDate: (date: Date) => void;
	state: CalendarEmbedPageState;
	text: CalendarLocaleText;
};

export function createCalendarPageController(context: CalendarPageControllerContext) {
	const draftEventPlaceholderTitle = () => context.text.newEvent;
	const draftEvents = new CalendarDraftEventState(draftEventPlaceholderTitle, () => context.text.newEvent);
	const programmaticUpdates = new CalendarProgrammaticUpdateState();
	const selectedMonthDate = createCalendarSelectedMonthDateActions({
		isBrowser: context.isBrowser,
		getStageElement: () => context.state.calendarStageElement,
		getSelectedDateKey: () => context.state.selectedMonthDateKey,
		setSelectedDateKey: (dateKey) => {
			context.state.selectedMonthDateKey = dateKey;
		}
	});

	const conflictActions = createCalendarConflictActions({
		isBrowser: context.isBrowser,
		errorFallback: () => context.text.error,
		getCalendarConflicts: () => context.state.calendarConflicts,
		setCalendarConflicts: (conflicts) => {
			context.state.calendarConflicts = conflicts;
		},
		setErrorMessage: () => {},
		syncRemoteCalendarAndRefresh: async () => {
			await renderSync.syncRemoteCalendarAndRefresh();
		}
	});

	const eventLoader: CalendarEventLoader = createCalendarEventLoader({
		isBrowser: context.isBrowser,
		errorFallback: () => context.text.error,
		getCalendarEvents: () => calendar.app.getAllEvents(),
		applyCalendarEventsChanges: (changes) => {
			calendar.app.applyEventsChanges(changes);
		},
		triggerCalendarRender: () => {
			calendar.app.triggerRender();
		},
		setVisibleEvents: (events) => {
			context.state.visibleEvents = events;
		},
		setEventCount: () => {},
		setIsLoading: () => {},
		setErrorMessage: () => {},
		refreshSelectedMonthDateCell: selectedMonthDate.refreshSelectedMonthDateCellAfterRender,
		preservedLocalEvents: () => draftEvents.createdEvents(),
		shouldPreserveLocalEvent: (event) => shouldPreserveLocalCalendarEvent(draftEvents, event),
		afterRenderEvents: (events) => {
			eventSelection.openPendingCalendarEvent(events);
			renderSync.scheduleCalendarEventDOMSync();
		}
	});

	const eventActions: CalendarEventActions = createCalendarEventActions(
		{
			isBrowser: context.isBrowser,
			getCurrentDate: () => calendar.currentDate,
			getStageElement: () => context.state.calendarStageElement,
			getSelectedAuditEventID: () => context.state.selectedAuditEventID,
			setSelectedAuditEventID: (eventID) => {
				context.state.selectedAuditEventID = eventID;
			},
			getCalendarEvents: () => calendar.app.getAllEvents(),
			addCalendarEvent: (event) => calendar.addEvent(event),
			restoreCalendarEvent: (event) => {
				calendar.app.applyEventsChanges({ delete: [event.id], add: [event] });
			},
			removeCalendarEvent: (eventID) => {
				calendar.app.applyEventsChanges({ delete: [eventID], add: [] });
			},
			updateCalendarEvent: async (eventID, changes, shouldRender) => {
				await calendar.updateEvent(eventID, changes, shouldRender);
			},
			setEventCount: () => {},
			setVisibleEvents: (events) => {
				context.state.visibleEvents = events;
			},
			setIsSaving: (nextIsSaving) => {
				context.state.isSaving = nextIsSaving;
			},
			setStatusMessage: () => {},
			setErrorMessage: () => {},
			openEventDetails: (eventID) => eventDetails.openEventDetails(eventID),
			openMobileEventEditor: (event) => {
				context.state.draftPopover = null;
				calendar.app.onMobileEventDetailToggle(event);
			},
			notifyEventsChanged: broadcastCalendarEventsChanged,
			refreshCalendar: async () => {
				await renderSync.refreshCalendar();
			},
			text: {
				get calendarTargetUnavailableError() {
					return context.text.calendarTargetUnavailableError;
				},
				get deleteError() {
					return context.text.deleteError;
				},
				get deleteUndoAction() {
					return context.text.deleteUndoAction;
				},
				get deleteUndoMessage() {
					return context.text.deleteUndoMessage;
				},
				get draftTitlePlaceholder() {
					return context.text.newEvent;
				},
				get saveError() {
					return context.text.saveError;
				},
				get shared() {
					return context.text.shared;
				}
			}
		},
		draftEvents,
		programmaticUpdates
	);

	const calendar = useCalendarApp(createCalendarPageAppOptions({
		defaultView: context.initialCalendarView(),
		initialDate: context.initialCalendarDate(),
		locale: context.getCalendarLocale(),
		text: context.text,
		getToolbarDate: () => context.state.toolbarDate,
		loadEvents: eventLoader.loadEvents,
		setVisibleDate: context.setVisibleDate,
		selectCalendarEvent: (eventID) => eventSelection.selectCalendarEvent(eventID),
		saveCreatedEvent: eventActions.saveCreatedEvent,
		saveUpdatedEvent: eventActions.saveUpdatedEvent,
		deleteEvent: eventActions.deleteEvent
	}));

	const eventSelection = createCalendarPageEventSelection({
		calendar,
		getStageElement: () => context.state.calendarStageElement,
		getPendingEventID: () => context.state.pendingEventID ?? '',
		setPendingEventID: (eventID) => {
			context.state.pendingEventID = eventID;
		},
		setSelectedAuditEventID: (eventID) => {
			context.state.selectedAuditEventID = eventID;
		},
		setVisibleEvents: (events) => {
			context.state.visibleEvents = events;
		},
		saveUpdatedEvent: (event, previousEvent) => eventActions.saveUpdatedEvent(event, previousEvent)
	});

	const draftPopoverActions: CalendarDraftPopoverActions = createCalendarDraftPopoverActions({
		eventActions,
		getDraftPopover: () => context.state.draftPopover,
		setDraftPopover: (popover) => {
			context.state.draftPopover = popover;
		},
		getCalendarEvents: () => calendar.events,
		getStageElement: () => context.state.calendarStageElement,
		selectEvent: eventSelection.selectCalendarEvent,
		replaceLocalEvent: eventSelection.replaceLocalCalendarEvent
	});

	const eventDetails = createCalendarPageEventDetails({
		getAppEvents: () => calendar.app.getAllEvents(),
		getFallbackEvents: () => calendar.events,
		openEventDraftPopover: draftPopoverActions.openEventDraftPopover
	});

	const renderSync = createCalendarPageRenderSync({
		isBrowser: context.isBrowser,
		errorFallback: () => context.text.error,
		getStageElement: () => context.state.calendarStageElement,
		getToolbarView: () => context.state.toolbarView,
		getToolbarDate: () => context.state.toolbarDate,
		getCalendarEvents: () => calendar.app.getAllEvents(),
		refreshCurrentRange: eventLoader.refreshCurrentRange,
		loadCalendarConflicts: conflictActions.loadCalendarConflicts,
		broadcastCalendarEventsChanged
	});

	const interactionServices = createCalendarPageInteractionServices({
		broadcastCalendarView,
		calendar,
		eventLoader,
		getLocaleCode: context.getLocaleCode,
		getIsMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView,
		isBrowser: context.isBrowser,
		selectedMonthDate,
		selectCalendarEvent: eventSelection.selectCalendarEvent,
		setVisibleDate: context.setVisibleDate,
		state: context.state
	});

	return {
		calendar,
		conflictActions,
		draftEvents,
		draftPopoverActions,
		eventActions,
		eventDetails,
		eventLoader,
		eventSelection,
		pageMessages: interactionServices.pageMessages,
		pageNavigation: interactionServices.pageNavigation,
		rangePreview: interactionServices.rangePreview,
		renderSync,
		scrollOverlays: interactionServices.scrollOverlays,
		selectedMonthDate
	};
}
