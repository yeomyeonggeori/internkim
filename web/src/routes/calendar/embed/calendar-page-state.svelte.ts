import type { Event as DayFlowEvent } from '@dayflow/core';
import type { ViewType } from '@dayflow/svelte';
import type { CalendarConflict } from './calendar-conflicts';
import type { DraftPopoverState } from './calendar-draft-popover-state';
import type { CalendarParticipant } from './calendar-participants';
import type { MonthRangePreviewSegment, MonthRangeSelection } from './calendar-month-range-action';
import type { VisibleMonthScrollLabel } from './calendar-month-scroll-overlay-state';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';
import type { TimelineRangePreviewSegment } from './calendar-timeline-preview';

type CalendarEmbedPageStateInitialValues = {
	toolbarDate: Date;
	toolbarView: ViewType;
	pendingEventID: string | null;
};

export class CalendarEmbedPageState {
	isSaving = $state(false);
	visibleEvents = $state<DayFlowEvent[]>([]);
	calendarStageElement = $state<HTMLElement | null>(null);
	monthRangeSelection = $state<MonthRangeSelection | null>(null);
	monthRangePreviewSegments = $state<MonthRangePreviewSegment[]>([]);
	timelineRangeSelection = $state<TimelineRangeSelection | null>(null);
	timelineRangePreviewSegments = $state<TimelineRangePreviewSegment[]>([]);
	monthScrollOverlayLabels = $state<VisibleMonthScrollLabel[]>([]);
	selectedMonthDateKey = $state<string | null>(null);
	searchText = $state('');
	toolbarDate: Date;
	toolbarView: ViewType;
	selectedAuditEventID = $state<string | null>(null);
	activeMobileEditorEventID = $state<string | null>(null);
	pendingEventID: string | null;
	draftPopover = $state<DraftPopoverState | null>(null);
	calendarConflicts = $state<CalendarConflict[]>([]);
	participantCandidates = $state<CalendarParticipant[]>([]);

	constructor(initialValues: CalendarEmbedPageStateInitialValues) {
		this.toolbarDate = $state(initialValues.toolbarDate);
		this.toolbarView = $state(initialValues.toolbarView);
		this.pendingEventID = $state(initialValues.pendingEventID);
	}
}

export function createCalendarEmbedPageState(
	initialValues: CalendarEmbedPageStateInitialValues
): CalendarEmbedPageState {
	return new CalendarEmbedPageState(initialValues);
}
