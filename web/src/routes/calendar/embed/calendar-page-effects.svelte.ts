import type { Event as DayFlowEvent } from '@dayflow/core';
import type { ViewType } from '@dayflow/svelte';
import type { CalendarLocaleText } from '../text';
import type { CalendarPageRangePreviewActions } from './calendar-page-range-preview';
import type { CalendarPageRenderSyncActions } from './calendar-page-render-sync';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';

type CalendarPageEffectsContext = {
	isBrowser: () => boolean;
	getMonthRangeSelection: () => MonthRangeSelection | null;
	getTimelineRangeSelection: () => TimelineRangeSelection | null;
	getStageElement: () => HTMLElement | null;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	getVisibleEvents: () => DayFlowEvent[];
	rangePreview: CalendarPageRangePreviewActions;
	renderSync: CalendarPageRenderSyncActions;
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
