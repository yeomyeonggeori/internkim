import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import type { ViewType } from '../calendar-view-type';
import { createCalendarEventStore } from './calendar-event-store.svelte';
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
import { createCalendarPageEventDetails } from './calendar-page-event-details';
import { createCalendarPageEventSelection } from './calendar-page-event-selection';
import { createCalendarPageInteractionServices } from './calendar-page-interaction-services';
import { createCalendarPageRenderSync } from './calendar-page-render-sync';
import type { CalendarEmbedPageState } from './calendar-page-state.svelte';
import type { CalendarHolidayLocale } from './calendar-holiday-persistence';

type CalendarPageControllerContext = {
	isBrowser: () => boolean;
	getIsMobileTwoDayWeekView: () => boolean;
	getLocaleCode: () => string;
	getLocale: () => CalendarHolidayLocale;
	initialCalendarDate: () => Date;
	initialCalendarView: () => ViewType;
	setVisibleDate: (date: Date) => void;
	state: CalendarEmbedPageState;
	text: CalendarLocaleText;
};

export function createCalendarPageController(context: CalendarPageControllerContext) {
	const draftEventPlaceholderTitle = () => context.text.newEvent;
	const draftEvents = new CalendarDraftEventState();
	const eventStore = createCalendarEventStore();
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
		getLocale: context.getLocale,
		errorFallback: () => context.text.error,
		getCalendarEvents: () => eventStore.getAllEvents(),
		getVisibleEvents: () => context.state.visibleEvents,
		applyCalendarEventsChanges: (changes) => {
			eventStore.applyEventsChanges(changes);
		},
		triggerCalendarRender: () => {},
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
		}
	});

	const eventActions: CalendarEventActions = createCalendarEventActions(
		{
			isBrowser: context.isBrowser,
			defaultEventParticipants: () => context.state.viewerParticipants,
			getCurrentDate: () => context.state.toolbarDate,
			getStageElement: () => context.state.calendarStageElement,
			getSelectedAuditEventID: () => context.state.selectedAuditEventID,
			setSelectedAuditEventID: (eventID) => {
				context.state.selectedAuditEventID = eventID;
			},
			getCalendarEvents: () => eventStore.getAllEvents(),
			addCalendarEvent: (event) => eventStore.addEvent(event),
			restoreCalendarEvent: (event) => {
				eventStore.applyEventsChanges({ delete: [event.id], add: [event] });
			},
			removeCalendarEvent: (eventID) => {
				eventStore.applyEventsChanges({ delete: [eventID], add: [] });
			},
			updateCalendarEvent: async (eventID, changes) => {
				eventStore.updateEvent(eventID, changes);
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
				context.state.activeMobileEditorEventID = event.id;
			},
			notifyEventsChanged: broadcastCalendarEventsChanged,
			invalidatePendingEventLoad: eventLoader.invalidatePendingLoad,
			refreshCalendar: async () => {
				await renderSync.refreshCalendar();
			},
			text: {
				get calendarDeleteVersionConflictError() {
					return context.text.calendarDeleteVersionConflictError;
				},
				get calendarEventVersionConflictError() {
					return context.text.calendarEventVersionConflictError;
				},
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

	const eventSelection = createCalendarPageEventSelection({
		eventStore,
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
		untitledEventTitle: draftEventPlaceholderTitle,
		getDraftPopover: () => context.state.draftPopover,
		setDraftPopover: (popover) => {
			context.state.draftPopover = popover;
		},
		getCalendarEvents: () => eventStore.events,
		getStageElement: () => context.state.calendarStageElement,
		selectEvent: eventSelection.selectCalendarEvent,
		replaceLocalEvent: eventSelection.replaceLocalCalendarEvent
	});

	const eventDetails = createCalendarPageEventDetails({
		getAppEvents: () => eventStore.getAllEvents(),
		getFallbackEvents: () => eventStore.events,
		openEventDraftPopover: draftPopoverActions.openEventDraftPopover
	});

	const renderSync = createCalendarPageRenderSync({
		isBrowser: context.isBrowser,
		errorFallback: () => context.text.error,
		getStageElement: () => context.state.calendarStageElement,
		getSelectedEventID: () => context.state.selectedAuditEventID,
		getToolbarView: () => context.state.toolbarView,
		getToolbarDate: () => context.state.toolbarDate,
		getCalendarEvents: () => eventStore.getAllEvents(),
		refreshCurrentRange: eventLoader.refreshCurrentRange,
		loadCalendarConflicts: conflictActions.loadCalendarConflicts,
		broadcastCalendarEventsChanged
	});

	const interactionServices = createCalendarPageInteractionServices({
		broadcastCalendarView,
		eventLoader,
		getLocaleCode: context.getLocaleCode,
		getIsMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView,
		isBrowser: context.isBrowser,
		selectedMonthDate,
		selectCalendarEvent: eventSelection.selectCalendarEvent,
		createQuickEvent: () => draftPopoverActions.createQuickDraftPopover(null),
		setVisibleDate: context.setVisibleDate,
		state: context.state
	});

	return {
		eventStore,
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
		selectedMonthDate
	};
}
