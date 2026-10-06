import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import type { ViewType } from '../calendar-view-type';
import type { DraftPopoverState } from './calendar-draft-popover-state';
import type { CalendarParticipant } from './calendar-participants';
import type { MonthRangePreviewSegment, MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';
import type { TimelineRangePreviewSegment } from './calendar-timeline-preview';

type CalendarEmbedPageStateInitialValues = {
	toolbarDate: Date;
	toolbarView: ViewType;
	pendingEventID: string | null;
};

export class CalendarEmbedPageState {
	isSaving = $state(false);
	isLoading = $state(true);
	isInitialLoading = $state(true);
	loadErrorMessage = $state('');
	visibleEvents = $state<DayTaskEvent[]>([]);
	calendarStageElement = $state<HTMLElement | null>(null);
	monthRangeSelection = $state<MonthRangeSelection | null>(null);
	monthRangePreviewSegments = $state<MonthRangePreviewSegment[]>([]);
	timelineRangeSelection = $state<TimelineRangeSelection | null>(null);
	timelineRangePreviewSegments = $state<TimelineRangePreviewSegment[]>([]);
	selectedMonthDateKey = $state<string | null>(null);
	searchText = $state('');
	participantFilterKey = $state('');
	toolbarDate: Date;
	toolbarView: ViewType;
	selectedAuditEventID = $state<string | null>(null);
	activeMobileEditorEventID = $state<string | null>(null);
	pendingEventID: string | null;
	draftPopover = $state<DraftPopoverState | null>(null);
	participantCandidates = $state<CalendarParticipant[]>([]);
	viewerParticipants = $state<CalendarParticipant[]>([]);

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
