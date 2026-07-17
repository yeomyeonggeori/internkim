import type { Event as DayFlowEvent } from '@dayflow/core';
import type { Locale } from '@dayflow/core';
import type { ViewType } from '@dayflow/svelte';
import type { CalendarLocaleText } from '../text';
import { miniCalendarWeekdayLabels } from './calendar-embed-view-helpers';
import {
	scheduleCalendarMobileTwoDayWeekLayoutSync,
	scheduleDayFlowMiniCalendarEnhancement
} from './calendar-embed-dom-sync';
import type { CalendarPageRangePreviewActions } from './calendar-page-range-preview';
import type { CalendarPageRenderSyncActions } from './calendar-page-render-sync';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import type { CalendarPageNavigation } from './calendar-page-navigation';
import type { MonthRangeSelection } from './calendar-month-range-action';
import type { TimelineRangeSelection } from './calendar-timeline-range-action';
import { installCalendarStageLayoutResizeSync } from './calendar-stage-layout-resize';

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
	getCurrentLocale: () => 'ko' | 'en';
	getIsMobileTwoDayWeekView: () => boolean;
	getMonthRangeSelection: () => MonthRangeSelection | null;
	getTimelineRangeSelection: () => TimelineRangeSelection | null;
	getStageElement: () => HTMLElement | null;
	getToolbarDate: () => Date;
	getToolbarView: () => ViewType;
	getVisibleEvents: () => DayFlowEvent[];
	getLocaleCode: () => string;
	getSelectedMonthDateKey: () => string | null;
	rangePreview: CalendarPageRangePreviewActions;
	renderSync: CalendarPageRenderSyncActions;
	selectedMonthDate: CalendarSelectedMonthDateActions;
	pageNavigation: CalendarPageNavigation;
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

	$effect(() => {
		const stageElement = context.getStageElement();
		const toolbarDate = context.getToolbarDate();
		const toolbarView = context.getToolbarView();
		const visibleEvents = context.getVisibleEvents();
		toolbarView;
		if (!context.isBrowser()) return;
		scheduleDayFlowMiniCalendarEnhancement({
			stageElement,
			currentDate: toolbarDate,
			events: visibleEvents,
			localeCode: context.getLocaleCode(),
			weekdayLabels: miniCalendarWeekdayLabels(context.getCurrentLocale()),
			previousLabel: context.text.previous,
			nextLabel: context.text.next,
			selectDateKey: context.pageNavigation.navigateToDateKey
		});
	});

	$effect(() => {
		context.getStageElement();
		context.getToolbarDate();
		context.getToolbarView();
		context.calendar.events;
		context.getVisibleEvents();
		context.renderSync.scheduleCalendarEventDOMSync();
	});

	$effect(() => {
		const stageElement = context.getStageElement();
		if (!context.isBrowser() || !stageElement) return;
		return installCalendarStageLayoutResizeSync(
			stageElement,
			context.renderSync.scheduleCalendarEventDOMSync
		);
	});

	$effect(() => {
		const stageElement = context.getStageElement();
		const toolbarDate = context.getToolbarDate();
		const isMobileTwoDayWeekView = context.getIsMobileTwoDayWeekView();
		const visibleEvents = context.getVisibleEvents();
		const localeCode = context.getLocaleCode();
		if (!context.isBrowser()) return;
		scheduleCalendarMobileTwoDayWeekLayoutSync({
			stageElement,
			currentDate: toolbarDate,
			events: visibleEvents,
			isMobileTwoDayWeekView,
			localeCode
		});
	});

	$effect(() => {
		context.getSelectedMonthDateKey();
		context.getStageElement();
		context.selectedMonthDate.refreshSelectedMonthDateCellAfterRender();
	});
}
