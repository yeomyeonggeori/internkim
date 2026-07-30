import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { ViewType } from '../calendar-view-type';
import {
	monthRangePreviewSegmentsFromSelection,
	type MonthRangePreviewSegment,
	type MonthRangeSelection
} from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';
import {
	clearTimelineOverlapLayout,
	syncTimelinePreviewOverlapLayout,
	timelineRangePreviewSegments,
	type TimelineRangePreviewSegment
} from './calendar-timeline-preview';

type CalendarPageRangePreviewContext = {
	getMonthRangeSelection: () => MonthRangeSelection | null;
	getStageElement: () => HTMLElement | null;
	getIsMobileTwoDayWeekView: () => boolean;
	getTimelineRangeSelection: () => TimelineRangeSelection | null;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	getVisibleEvents: () => DayFlowEvent[];
	isBrowser: () => boolean;
	setMonthRangePreviewSegments: (segments: MonthRangePreviewSegment[]) => void;
	setTimelineRangePreviewSegments: (segments: TimelineRangePreviewSegment[]) => void;
	setTimelineRangeSelection: (selection: TimelineRangeSelection | null) => void;
};

export type CalendarPageRangePreviewActions = {
	refreshMonthRangePreview: () => void;
	refreshTimelineRangePreview: () => void;
	setTimelineRangeSelection: (selection: TimelineRangeSelection | null) => void;
};

export function createCalendarPageRangePreview(
	context: CalendarPageRangePreviewContext
): CalendarPageRangePreviewActions {
	function refreshMonthRangePreview(): void {
		context.setMonthRangePreviewSegments(
			monthRangePreviewSegmentsFromSelection(context.getStageElement(), context.getMonthRangeSelection())
		);
	}

	function setTimelineRangeSelection(selection: TimelineRangeSelection | null): void {
		context.setTimelineRangeSelection(selection);
		refreshTimelineRangePreview();
	}

	function refreshTimelineRangePreview(): void {
		updateTimelineRangePreviewSegments();
		scheduleTimelinePreviewOverlapLayoutSync();
	}

	function updateTimelineRangePreviewSegments(): void {
		context.setTimelineRangePreviewSegments(
			timelineRangePreviewSegments({
				stageElement: context.getStageElement(),
				selection: context.getTimelineRangeSelection(),
				currentView: context.getToolbarView(),
				currentDate: context.getToolbarDate(),
				isMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView(),
				events: context.getVisibleEvents()
			})
		);
	}

	function scheduleTimelinePreviewOverlapLayoutSync(): void {
		if (!context.isBrowser()) return;
		syncTimelinePreviewOverlapLayout(
			context.getStageElement(),
			context.getTimelineRangeSelection(),
			context.getVisibleEvents()
		);
		requestAnimationFrame(() => {
			updateTimelineRangePreviewSegments();
			syncTimelinePreviewOverlapLayout(
				context.getStageElement(),
				context.getTimelineRangeSelection(),
				context.getVisibleEvents()
			);
		});
		if (!context.getTimelineRangeSelection()) {
			requestAnimationFrame(() => clearTimelineOverlapLayout(context.getStageElement()));
		}
	}

	return {
		refreshMonthRangePreview,
		refreshTimelineRangePreview,
		setTimelineRangeSelection
	};
}
