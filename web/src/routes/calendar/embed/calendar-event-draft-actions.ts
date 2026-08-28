import type { CalendarModelEvent as DayTaskEvent } from './calendar-event-model';
import {
	allDaySingleDraftEventParams,
	monthRangeDraftEventParams,
	monthSingleDayDraftEventParams,
	quickDraftEventParams,
	timelineRangeDraftEventParams,
	timelineSingleDraftEventParams
} from './calendar-draft-event-params';
import type { DraftEventParams } from './calendar-draft-events';
import type { MonthRangeSelection } from './calendar-month-range-action';

type CalendarEventDraftActionsContext = {
	addDraftEvent: (params: DraftEventParams) => DayTaskEvent;
	getCurrentDate: () => Date | null | undefined;
};

export type CalendarEventDraftActions = {
	createAllDaySingleEvent: (dateKey: string) => DayTaskEvent | null;
	createMonthRangeEvent: (selection: MonthRangeSelection) => DayTaskEvent;
	createMonthSingleDayEvent: (dateKey: string) => DayTaskEvent | null;
	createQuickEvent: () => DayTaskEvent;
	createTimelineRangeEvent: (firstDate: Date, secondDate: Date) => DayTaskEvent | null;
	createTimelineSingleEvent: (startDate: Date) => DayTaskEvent | null;
};

export function createCalendarEventDraftActions(
	context: CalendarEventDraftActionsContext
): CalendarEventDraftActions {
	let lastDateCellCreationTime = 0;
	let lastTimelineSlotCreationTime = 0;

	function createQuickEvent(): DayTaskEvent {
		const baseDate = context.getCurrentDate() ?? new Date();
		return context.addDraftEvent(quickDraftEventParams(baseDate));
	}

	function createMonthRangeEvent(selection: MonthRangeSelection): DayTaskEvent {
		const params = monthRangeDraftEventParams(selection);
		if (!params) {
			return context.addDraftEvent(monthSingleDayDraftEventParams(selection.startDateKey));
		}
		return context.addDraftEvent(params);
	}

	function createMonthSingleDayEvent(dateKey: string): DayTaskEvent | null {
		if (Date.now() - lastDateCellCreationTime < 250) return null;
		lastDateCellCreationTime = Date.now();
		return context.addDraftEvent(monthSingleDayDraftEventParams(dateKey));
	}

	function createAllDaySingleEvent(dateKey: string): DayTaskEvent | null {
		if (Date.now() - lastDateCellCreationTime < 250) return null;
		lastDateCellCreationTime = Date.now();
		return context.addDraftEvent(allDaySingleDraftEventParams(dateKey));
	}

	function createTimelineSingleEvent(startDate: Date): DayTaskEvent | null {
		if (Date.now() - lastTimelineSlotCreationTime < 80) return null;
		lastTimelineSlotCreationTime = Date.now();
		return context.addDraftEvent(timelineSingleDraftEventParams(startDate));
	}

	function createTimelineRangeEvent(firstDate: Date, secondDate: Date): DayTaskEvent | null {
		if (Date.now() - lastTimelineSlotCreationTime < 80) return null;
		lastTimelineSlotCreationTime = Date.now();
		return context.addDraftEvent(timelineRangeDraftEventParams(firstDate, secondDate));
	}

	return {
		createAllDaySingleEvent,
		createMonthRangeEvent,
		createMonthSingleDayEvent,
		createQuickEvent,
		createTimelineRangeEvent,
		createTimelineSingleEvent
	};
}
