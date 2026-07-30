import { tick } from 'svelte';
import type { ViewType } from '../calendar-view-type';
import type { CalendarLocaleText } from '../text';
import type { DraftPopoverAnchor, DraftPopoverState } from './calendar-draft-popover-state';
import { installCalendarEmbedLifecycle } from './calendar-embed-lifecycle';
import type { CalendarDraftPopoverActions } from './calendar-draft-popover-actions';
import type { CalendarEventActions } from './calendar-event-actions';
import type { CalendarEventLoader } from './calendar-event-loader';
import type { CalendarPageEventSelectionActions } from './calendar-page-event-selection';
import { createCalendarPageLifecycleOptions } from './calendar-page-lifecycle-options';
import type { CalendarPageMessageActions } from './calendar-page-messages';
import type { CalendarPageNavigation } from './calendar-page-navigation';
import type { CalendarPageRangePreviewActions } from './calendar-page-range-preview';
import type { CalendarPageRenderSyncActions } from './calendar-page-render-sync';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';

type CalendarPageLifecycleInstallContext = {
	applyCalendarView: (view: ViewType) => void;
	clearDraftPopover: () => void;
	deleteEvent: (eventID: string) => Promise<void>;
	undoLastDelete: () => boolean;
	draftPopoverActions: CalendarDraftPopoverActions;
	eventActions: CalendarEventActions;
	eventLoader: CalendarEventLoader;
	eventSelection: CalendarPageEventSelectionActions;
	getCurrentView: () => ViewType;
	getDraftPopover: () => DraftPopoverState | null;
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

export function installCalendarPageLifecycle(context: CalendarPageLifecycleInstallContext): () => void {
	let stopLifecycle: (() => void) | null = null;
	let isDisposed = false;
	void tick().then(() => {
		if (isDisposed) return;
		stopLifecycle = installCalendarEmbedLifecycle(createCalendarPageLifecycleOptions(context));
	});
	return () => {
		isDisposed = true;
		stopLifecycle?.();
	};
}
