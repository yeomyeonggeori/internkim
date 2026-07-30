import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { ViewType } from '../calendar-view-type';
import type { CalendarViewValue } from '../calendar-navigation-message';
import type { CalendarEventLoader } from './calendar-event-loader';
import type { CalendarSelectedMonthDateActions } from './calendar-month-selection';
import { createCalendarPageMessageActions } from './calendar-page-messages';
import { createCalendarPageNavigation } from './calendar-page-navigation';
import { createCalendarPageRangePreview } from './calendar-page-range-preview';
import type { CalendarEmbedPageState } from './calendar-page-state.svelte';

type CalendarPageInteractionServicesContext = {
	broadcastCalendarView: (view: CalendarViewValue) => void;
	eventLoader: CalendarEventLoader;
	getLocaleCode: () => string;
	getIsMobileTwoDayWeekView: () => boolean;
	isBrowser: () => boolean;
	selectedMonthDate: CalendarSelectedMonthDateActions;
	selectCalendarEvent: (eventID: string) => void;
	createQuickEvent: () => void;
	setVisibleDate: (date: Date) => void;
	state: CalendarEmbedPageState;
};

export function createCalendarPageInteractionServices(context: CalendarPageInteractionServicesContext) {
	const pageNavigation = createCalendarPageNavigation({
		getToolbarDate: () => context.state.toolbarDate,
		getToolbarView: () => context.state.toolbarView,
		setToolbarView: (view) => {
			context.state.toolbarView = view;
		},
		setVisibleDate: context.setVisibleDate,
		setSelectedMonthDateKey: (dateKey) => {
			context.state.selectedMonthDateKey = dateKey;
		},
		selectCalendarEvent: context.selectCalendarEvent,
		broadcastCalendarView: context.broadcastCalendarView,
		isMobileTwoDayWeekView: context.getIsMobileTwoDayWeekView
	});

	const pageMessages = createCalendarPageMessageActions({
		getCurrentOrigin: () => window.location.origin,
		navigateToDateKey: pageNavigation.navigateToDateKey,
		createQuickEvent: context.createQuickEvent
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

	return {
		pageMessages,
		pageNavigation,
		rangePreview,
	};
}
