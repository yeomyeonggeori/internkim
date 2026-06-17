import type { Event as DayFlowEvent } from '@dayflow/core';
import {
	monthRangeDraftEventParams,
	monthSingleDayDraftEventParams,
	quickDraftEventParams,
	timelineRangeDraftEventParams,
	timelineSingleDraftEventParams
} from './calendar-draft-event-params';
import type { DraftEventParams } from './calendar-draft-events';
import type { MonthRangeSelection } from './calendar-month-range-action';

type CalendarEventDraftActionsContext = {
	addDraftEvent: (params: DraftEventParams) => DayFlowEvent;
	getCurrentDate: () => Date | null | undefined;
};

export type CalendarEventDraftActions = {
	createMonthRangeEvent: (selection: MonthRangeSelection) => DayFlowEvent;
	createMonthSingleDayEvent: (dateKey: string) => DayFlowEvent | null;
	createQuickEvent: () => DayFlowEvent;
	createTimelineRangeEvent: (firstDate: Date, secondDate: Date) => DayFlowEvent | null;
	createTimelineSingleEvent: (startDate: Date) => DayFlowEvent | null;
};

export function createCalendarEventDraftActions(
	context: CalendarEventDraftActionsContext
): CalendarEventDraftActions {
	let lastMonthCellCreationTime = 0;
	let lastTimelineSlotCreationTime = 0;

	function createQuickEvent(): DayFlowEvent {
		const baseDate = context.getCurrentDate() ?? new Date();
		return context.addDraftEvent(quickDraftEventParams(baseDate));
	}

	function createMonthRangeEvent(selection: MonthRangeSelection): DayFlowEvent {
		const params = monthRangeDraftEventParams(selection);
		if (!params) {
			return context.addDraftEvent(monthSingleDayDraftEventParams(selection.startDateKey));
		}
		return context.addDraftEvent(params);
	}

	function createMonthSingleDayEvent(dateKey: string): DayFlowEvent | null {
		if (Date.now() - lastMonthCellCreationTime < 250) return null;
		lastMonthCellCreationTime = Date.now();
		return context.addDraftEvent(monthSingleDayDraftEventParams(dateKey));
	}

	function createTimelineSingleEvent(startDate: Date): DayFlowEvent | null {
		if (Date.now() - lastTimelineSlotCreationTime < 80) return null;
		lastTimelineSlotCreationTime = Date.now();
		return context.addDraftEvent(timelineSingleDraftEventParams(startDate));
	}

	function createTimelineRangeEvent(firstDate: Date, secondDate: Date): DayFlowEvent | null {
		if (Date.now() - lastTimelineSlotCreationTime < 80) return null;
		lastTimelineSlotCreationTime = Date.now();
		return context.addDraftEvent(timelineRangeDraftEventParams(firstDate, secondDate));
	}

	return {
		createMonthRangeEvent,
		createMonthSingleDayEvent,
		createQuickEvent,
		createTimelineRangeEvent,
		createTimelineSingleEvent
	};
}
