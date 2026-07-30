import type { CalendarModelEvent as DayFlowEvent } from '../embed/calendar-event-model';
import { eventEndDate, eventStartDate } from '../embed/calendar-event-mapping';
import { calendarParticipantsFromUnknown } from '../embed/calendar-participants';
import { addCalendarGridDays, startOfCalendarGridDay } from './calendar-grid-dates';
import type { CalendarGridEvent } from './calendar-grid-layout';

export const defaultCalendarEventColor = '#3b82f6';

export function calendarGridEventFromDayFlowEvent(event: DayFlowEvent): CalendarGridEvent {
	const isAllDay = event.allDay ?? false;
	const start = eventStartDate(event);
	const end = eventEndDate(event);
	return {
		id: event.id,
		title: event.title || '',
		start: isAllDay ? startOfCalendarGridDay(start) : start,
		end: isAllDay ? addCalendarGridDays(startOfCalendarGridDay(end), 1) : end,
		isAllDay,
		color: typeof event.meta?.color === 'string' && event.meta.color ? event.meta.color : defaultCalendarEventColor,
		participants: calendarParticipantsFromUnknown(event.meta?.participants),
		readOnly: event.calendarId === 'holidays' || event.meta?.readOnly === true,
		displayPriority: event.calendarId === 'holidays' ? 0 : 1
	};
}

export function calendarGridEventsFromDayFlowEvents(events: DayFlowEvent[]): CalendarGridEvent[] {
	return events.map(calendarGridEventFromDayFlowEvent);
}
