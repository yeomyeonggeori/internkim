import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import type { ViewType } from '../calendar-view-type';
import type { CalendarLocaleText } from '../text';
import type { CalendarPageRangePreviewActions } from './calendar-page-range-preview';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';

type CalendarPageEffectsContext = {
	isBrowser: () => boolean;
	getMonthRangeSelection: () => MonthRangeSelection | null;
	getTimelineRangeSelection: () => TimelineRangeSelection | null;
	getStageElement: () => HTMLElement | null;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	getVisibleEvents: () => DayTaskEvent[];
	rangePreview: CalendarPageRangePreviewActions;
	text: CalendarLocaleText;
};

export function installCalendarPageEffects(context: CalendarPageEffectsContext): void {
	$effect(() => {
		context.getMonthRangeSelection();
		context.getStageElement();
		context.rangePreview.refreshMonthRangePreview();
	});

	$effect(() => {
		context.getStageElement();
		context.getTimelineRangeSelection();
		context.getToolbarDate();
		context.getToolbarView();
		context.getVisibleEvents();
		context.rangePreview.refreshTimelineRangePreview();
	});

}
