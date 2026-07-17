import { tick } from 'svelte';
import type { ViewType } from '@dayflow/svelte';
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
import type { CalendarPageScrollOverlayActions } from './calendar-page-scroll-overlays';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';

type CalendarPageLifecycleInstallContext = {
	applyCalendarView: (view: ViewType) => void;
	clearDraftPopover: () => void;
	clearMonthRangePreview: () => void;
	deleteEvent: (eventID: string) => Promise<void>;
	draftPopoverActions: CalendarDraftPopoverActions;
	eventActions: CalendarEventActions;
	eventLoader: CalendarEventLoader;
	eventSelection: CalendarPageEventSelectionActions;
	getCurrentView: () => ViewType;
	getDraftPopover: () => DraftPopoverState | null;
	getLocaleCode: () => string;
	getIsMobileTwoDayWeekView: () => boolean;
	getMonthRangeSelection: () => MonthRangeSelection | null;
	getSelectedAuditEventID: () => string | null;
	getStageElement: () => HTMLElement | null;
	getTimelineRangeSelection: () => TimelineRangeSelection | null;
	getToolbarDate: () => Date;
	initialCalendarDate: () => Date;
	initialCalendarView: () => ViewType;
	openEventEditor: (eventID: string, anchor: DraftPopoverAnchor) => void;
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
