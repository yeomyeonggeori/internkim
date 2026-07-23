import type { ViewType } from '@dayflow/svelte';
import type { CalendarLocaleText } from '../text';
import type { CalendarDraftPopoverActions } from './calendar-draft-popover-actions';
import { dateKey, type DraftPopoverState } from './calendar-draft-popover-state';
import type { CalendarEmbedLifecycleOptions } from './calendar-embed-lifecycle';
import type { CalendarEventActions } from './calendar-event-actions';
import type { CalendarEventLoader } from './calendar-event-loader';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';
import type { CalendarPageEventSelectionActions } from './calendar-page-event-selection';
import type { CalendarPageMessageActions } from './calendar-page-messages';
import type { CalendarPageNavigation } from './calendar-page-navigation';
import type { CalendarPageRangePreviewActions } from './calendar-page-range-preview';
import type { CalendarPageRenderSyncActions } from './calendar-page-render-sync';
import type { CalendarPageScrollOverlayActions } from './calendar-page-scroll-overlays';

type CalendarPageLifecycleOptionsContext = {
	applyCalendarView: (view: ViewType) => void;
	clearDraftPopover: () => void;
	clearMonthRangePreview: () => void;
	deleteEvent: CalendarEventActions['deleteEvent'];
	draftPopoverActions: CalendarDraftPopoverActions;
	eventActions: CalendarEventActions;
	eventLoader: CalendarEventLoader;
	eventSelection: CalendarPageEventSelectionActions;
	getDraftPopover: () => DraftPopoverState | null;
	getCurrentView: () => ViewType;
	getLocaleCode: () => string;
	getIsMobileTwoDayWeekView: () => boolean;
	getMonthRangeSelection: () => MonthRangeSelection | null;
	getSelectedAuditEventID: () => string | null;
	getStageElement: () => HTMLElement | null;
	getTimelineRangeSelection: () => TimelineRangeSelection | null;
	getToolbarDate: () => Date;
	initialCalendarDate: () => Date;
	initialCalendarView: () => ViewType;
	openEventEditor: CalendarEmbedLifecycleOptions['eventKeyboardActivation']['openEvent'];
	pageMessages: CalendarPageMessageActions;
	pageNavigation: CalendarPageNavigation;
	rangePreview: CalendarPageRangePreviewActions;
	renderSync: CalendarPageRenderSyncActions;
	scrollOverlays: CalendarPageScrollOverlayActions;
	selectedMonthDate: CalendarSelectedMonthDateActions;
	setMonthRangeSelection: (selection: MonthRangeSelection | null) => void;
	setSelectedAuditEventID: (eventID: string | null) => void;
	setToolbarView: (view: ViewType) => void;
	syncCalendarThemeToDocument: () => void;
	text: CalendarLocaleText;
};

export function createCalendarPageLifecycleOptions(
	context: CalendarPageLifecycleOptionsContext
): CalendarEmbedLifecycleOptions {
	return {
		stageElement: context.getStageElement(),
		handleCalendarChannelMessage: context.pageMessages.handleCalendarChannelMessage,
		handleCalendarWindowMessage: context.pageMessages.handleCalendarWindowMessage,
		handleCalendarStorageMessage: context.pageMessages.handleCalendarStorageMessage,
		initialCalendarView: context.initialCalendarView,
		applyCalendarView: context.applyCalendarView,
		setToolbarView: context.setToolbarView,
		syncCalendarThemeToDocument: context.syncCalendarThemeToDocument,
		hasVisibleRange: context.eventLoader.hasVisibleRange,
		initialCalendarDate: context.initialCalendarDate,
		loadEvents: context.eventLoader.loadEvents,
		syncRemoteCalendarAndRefresh: context.renderSync.syncRemoteCalendarAndRefresh,
		scheduleDraftTitleInputPlaceholderUpdates: context.eventActions.scheduleDraftTitleInputPlaceholderUpdates,
		scheduleDraftEventVisibilitySync: context.eventActions.scheduleDraftEventVisibilitySync,
		refreshSelectedMonthDateCellAfterRender: context.selectedMonthDate.refreshSelectedMonthDateCellAfterRender,
		clearMonthScrollOverlays: context.scrollOverlays.clearMonthScrollOverlays,
		monthRangeAction: monthRangeAction(context),
		allDayCellAction: allDayCellAction(context),
		timelineRangeAction: timelineRangeAction(context),
		wheelNavigation: wheelNavigation(context),
		draftPopoverDismiss: draftPopoverDismiss(context),
		eventKeyboardActivation: {
			openEvent: context.openEventEditor
		},
		eventSelection: {
			clearSelectedEvent: context.eventSelection.clearSelectedEvent,
			selectEvent: context.eventSelection.selectCalendarEvent
		},
		monthKeyboardNavigation: {
			currentView: context.getCurrentView,
			getSelectedDateKey: () => context.selectedMonthDate.getSelectedMonthDateKey(),
			navigateToDateKey: context.pageNavigation.navigateToDateKey,
			clearSelectedEvent: context.eventSelection.clearSelectedEvent
		},
		keyboardDelete: keyboardDelete(context),
		miniCalendarMonthPicker: miniCalendarMonthPicker(context),
		navigateToDateKey: context.pageNavigation.navigateToDateKey
	};
}

function allDayCellAction(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['allDayCellAction'] {
	return {
		currentView: context.getCurrentView,
		currentDate: context.getToolbarDate,
		createAllDayEvent: context.draftPopoverActions.openAllDaySingleDraftPopover,
		createMobileAllDayEvent: context.eventActions.openAllDaySingleEventMobileEditor,
		isMobileEventEditor: context.getIsMobileTwoDayWeekView
	};
}

function monthRangeAction(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['monthRangeAction'] {
	return {
		currentView: context.getCurrentView,
		getSelection: context.getMonthRangeSelection,
		setSelection: context.setMonthRangeSelection,
		clearPreview: context.clearMonthRangePreview,
		selectDate: context.selectedMonthDate.selectMonthDate,
		createSingleDayEvent: context.draftPopoverActions.openMonthSingleDayDraftPopover,
		createRangeEvent: context.draftPopoverActions.openMonthRangeDraftPopover
	};
}

function timelineRangeAction(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['timelineRangeAction'] {
	return {
		currentView: context.getCurrentView,
		currentDate: context.getToolbarDate,
		isMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView,
		getSelection: context.getTimelineRangeSelection,
		setSelection: context.rangePreview.setTimelineRangeSelection,
		createSingleEvent: context.draftPopoverActions.openTimelineSingleDraftPopover,
		createMobileSingleEvent: context.eventActions.openTimelineSingleEventMobileEditor,
		createRangeEvent: context.draftPopoverActions.openTimelineRangeDraftPopover
	};
}

function wheelNavigation(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['wheelNavigation'] {
	return {
		currentView: context.getCurrentView,
		showMonthLabels: context.scrollOverlays.showMonthScrollOverlays,
		hideMonthLabels: context.scrollOverlays.clearMonthScrollOverlays,
		selectVisibleDate: context.pageNavigation.setVisibleDate
	};
}

function draftPopoverDismiss(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['draftPopoverDismiss'] {
	return {
		getDraftPopover: context.getDraftPopover,
		saveDraftPopover: context.draftPopoverActions.saveDraftPopover,
		cancelDraftPopover: context.draftPopoverActions.cancelDraftPopover,
		clearSelectedEvent: context.eventSelection.clearSelectedEvent
	};
}

function keyboardDelete(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['keyboardDelete'] {
	return {
		getSelectedEventID: context.getSelectedAuditEventID,
		getDraftPopover: context.getDraftPopover,
		deleteSelectedEvent: (eventID) => {
			context.setSelectedAuditEventID(null);
			context.clearDraftPopover();
			void context.deleteEvent(eventID);
		}
	};
}

function miniCalendarMonthPicker(context: CalendarPageLifecycleOptionsContext): CalendarEmbedLifecycleOptions['miniCalendarMonthPicker'] {
	return {
		getStageElement: context.getStageElement,
		getCurrentDate: context.getToolbarDate,
		localeCode: context.getLocaleCode,
		labels: () => ({
			pickMonthAndYear: context.text.pickMonthAndYear,
			previousYear: context.text.previousYear,
			nextYear: context.text.nextYear,
			previousTwelveYears: context.text.previousTwelveYears,
			nextTwelveYears: context.text.nextTwelveYears
		}),
		selectDate: (date) => {
			context.pageNavigation.navigateToDateKey(dateKey(date));
		}
	};
}
