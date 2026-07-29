import type { Event as DayFlowEvent } from '@dayflow/core';
import type { Locale } from '@dayflow/core';
import type { ViewType } from '@dayflow/svelte';
import type { CalendarLocaleText } from '../text';
import type { CalendarPageRangePreviewActions } from './calendar-page-range-preview';
import type { CalendarPageRenderSyncActions } from './calendar-page-render-sync';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';

type CalendarPageEffectsCalendar = {
	app: {
		triggerRender: () => void;
		updateConfig: (config: { locale: Locale }) => void;
	};
	events: DayFlowEvent[];
};

type CalendarPageEffectsContext = {
	isBrowser: () => boolean;
	calendar: CalendarPageEffectsCalendar;
	getCalendarLocale: () => Locale;
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
		context.calendar.app.updateConfig({ locale: context.getCalendarLocale() });
		context.calendar.app.triggerRender();
	});

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
