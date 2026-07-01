import type { Event as DayFlowEvent } from '@dayflow/core';
import { ViewType } from '@dayflow/svelte';
import type { CalendarViewValue } from '../calendar-navigation-message';
import type { CalendarEventLoader } from './calendar-event-loader';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import { createCalendarPageMessageActions } from './calendar-page-messages';
import { createCalendarPageNavigation } from './calendar-page-navigation';
import { createCalendarPageRangePreview } from './calendar-page-range-preview';
import { createCalendarPageScrollOverlays } from './calendar-page-scroll-overlays';
import type { CalendarEmbedPageState } from './calendar-page-state.svelte';

type CalendarPageInteractionCalendar = {
	changeView: (viewType: ViewType) => void;
	goToToday: () => void;
	goToPrevious: () => void;
	goToNext: () => void;
	app: {
		selectDate: (date: Date) => void;
		setCurrentDate: (date: Date) => void;
		setVisibleMonth: (date: Date) => void;
	};
};

type CalendarPageInteractionServicesContext = {
	broadcastCalendarView: (view: CalendarViewValue) => void;
	calendar: CalendarPageInteractionCalendar;
	eventLoader: CalendarEventLoader;
	getLocaleCode: () => string;
	getIsMobileTwoDayWeekView: () => boolean;
	isBrowser: () => boolean;
	selectedMonthDate: CalendarSelectedMonthDateActions;
	selectCalendarEvent: (eventID: string) => void;
	setVisibleDate: (date: Date) => void;
	state: CalendarEmbedPageState;
};

export function createCalendarPageInteractionServices(context: CalendarPageInteractionServicesContext) {
	const pageNavigation = createCalendarPageNavigation({
		calendar: context.calendar,
		getToolbarDate: () => context.state.toolbarDate,
		getToolbarView: () => context.state.toolbarView,
		setToolbarView: (view) => {
			context.state.toolbarView = view;
		},
		setVisibleDate: context.setVisibleDate,
		setSelectedMonthDateKey: (dateKey) => {
			context.state.selectedMonthDateKey = dateKey;
		},
		refreshSelectedMonthDateCellAfterRender: context.selectedMonthDate.refreshSelectedMonthDateCellAfterRender,
		selectCalendarEvent: context.selectCalendarEvent,
		broadcastCalendarView: context.broadcastCalendarView,
		isMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView
	});

	const pageMessages = createCalendarPageMessageActions({
		getCurrentOrigin: () => window.location.origin,
		navigateToDateKey: pageNavigation.navigateToDateKey
	});

	const rangePreview = createCalendarPageRangePreview({
		isBrowser: context.isBrowser,
		getStageElement: () => context.state.calendarStageElement,
		getMonthRangeSelection: () => context.state.monthRangeSelection,
		setMonthRangePreviewSegments: (segments) => {
			context.state.monthRangePreviewSegments = segments;
		},
		getTimelineRangeSelection: () => context.state.timelineRangeSelection,
		setTimelineRangeSelection: (selection) => {
			context.state.timelineRangeSelection = selection;
		},
		setTimelineRangePreviewSegments: (segments) => {
			context.state.timelineRangePreviewSegments = segments;
		},
		getToolbarView: () => context.state.toolbarView,
		getToolbarDate: () => context.state.toolbarDate,
		getIsMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView,
		getVisibleEvents: () => context.state.visibleEvents
	});

	const scrollOverlays = createCalendarPageScrollOverlays({
		getLocaleCode: context.getLocaleCode,
		getMonthScrollOverlayLabels: () => context.state.monthScrollOverlayLabels,
		setMonthScrollOverlayLabels: (labels) => {
			context.state.monthScrollOverlayLabels = labels;
		}
	});

	return {
		pageMessages,
		pageNavigation,
		rangePreview,
		scrollOverlays
	};
}
