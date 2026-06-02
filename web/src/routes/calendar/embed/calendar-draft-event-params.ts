import type { DraftEventParams } from './calendar-draft-events';
import { orderedDateKeys, type MonthRangeSelection } from './calendar-month-range-action';
import { dateFromDateKey } from './calendar-month-selection';

export function quickDraftEventParams(baseDate: Date): DraftEventParams {
	const startDate = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate(), 9, 0, 0, 0);
	const endDate = new Date(baseDate.getFullYear(), baseDate.getMonth(), baseDate.getDate(), 10, 0, 0, 0);
	return {
		id: draftEventID('quick'),
		start: startDate,
		end: endDate,
		allDay: false,
		calendarId: 'internkim'
	};
}

export function monthRangeDraftEventParams(selection: MonthRangeSelection): DraftEventParams | null {
	const [startDateKey, endDateKey] = orderedDateKeys(selection.startDateKey, selection.endDateKey);
	if (startDateKey === endDateKey) return null;
	return {
		id: draftEventID('range'),
		start: dateFromDateKey(startDateKey),
		end: dateFromDateKey(endDateKey),
		allDay: true,
		calendarId: 'internkim'
	};
}

export function monthSingleDayDraftEventParams(dateKey: string): DraftEventParams {
	const startDate = dateFromDateKey(dateKey);
	startDate.setHours(9, 0, 0, 0);
	const endDate = new Date(startDate);
	endDate.setHours(10, 0, 0, 0);
	return {
		id: draftEventID('month'),
		start: startDate,
		end: endDate,
		allDay: false,
		calendarId: 'internkim'
	};
}

function draftEventID(prefix: string): string {
	return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}
