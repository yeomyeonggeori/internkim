import type { ViewType } from '../calendar-view-type';
import type { CalendarLocaleText } from '../text';
import type { CalendarDraftPopoverActions } from './calendar-draft-popover-actions';
import { dateKey, type DraftPopoverState } from './calendar-draft-popover-state';
import type { CalendarEmbedLifecycleOptions } from './calendar-embed-lifecycle';
import type { CalendarEventActions } from './calendar-event-actions';
import type { CalendarEventLoader } from './calendar-event-loader';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import type { CalendarPageEventSelectionActions } from './calendar-page-event-selection';
import type { CalendarPageMessageActions } from './calendar-page-messages';
import type { CalendarPageNavigation } from './calendar-page-navigation';
import type { CalendarPageRenderSyncActions } from './calendar-page-render-sync';

type CalendarPageLifecycleOptionsContext = {
	applyCalendarView: (view: ViewType) => void;
	clearDraftPopover: () => void;
	deleteEvent: CalendarEventActions['deleteEvent'];
	undoLastDelete: CalendarEventActions['undoLastDelete'];
	draftPopoverActions: CalendarDraftPopoverActions;
	eventActions: CalendarEventActions;
	eventLoader: CalendarEventLoader;
	eventSelection: CalendarPageEventSelectionActions;
	getDraftPopover: () => DraftPopoverState | null;
	getCurrentView: () => ViewType;
	getSelectedAuditEventID: () => string | null;
	getStageElement: () => HTMLElement | null;
	getToolbarDate: () => Date;
	initialCalendarDate: () => Date;
	initialCalendarView: () => ViewType;
	pageMessages: CalendarPageMessageActions;
	pageNavigation: CalendarPageNavigation;
	renderSync: CalendarPageRenderSyncActions;
	selectedMonthDate: CalendarSelectedMonthDateActions;
	setSelectedAuditEventID: (eventID: string | null) => void;
	setToolbarView: (view: ViewType) => void;
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
		hasVisibleRange: context.eventLoader.hasVisibleRange,
		initialCalendarDate: context.initialCalendarDate,
		loadEvents: context.eventLoader.loadEvents,
		syncRemoteCalendarAndRefresh: context.renderSync.syncRemoteCalendarAndRefresh,
		draftPopoverDismiss: draftPopoverDismiss(context),
		monthKeyboardNavigation: {
			currentView: context.getCurrentView,
			getSelectedDateKey: () => context.selectedMonthDate.getSelectedMonthDateKey(),
			navigateToDateKey: context.pageNavigation.navigateToDateKey,
			clearSelectedEvent: context.eventSelection.clearSelectedEvent
		},
		keyboardDelete: keyboardDelete(context),
		keyboardUndo: { undoLastDelete: context.undoLastDelete },
		keyboardSave: {
			getDraftPopover: context.getDraftPopover,
			saveDraftPopover: context.draftPopoverActions.saveDraftPopover
		},
		navigateToDateKey: context.pageNavigation.navigateToDateKey
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

