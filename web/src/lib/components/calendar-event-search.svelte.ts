import { dayFlowEventFromCalendarEvent } from '../../routes/calendar/embed/calendar-event-mapping';
import { fetchCalendarEvents } from '../../routes/calendar/embed/calendar-event-persistence';
import { searchCalendarEvents, type CalendarSearchResult } from '../../routes/calendar/embed/calendar-search';
import type { Event as DayFlowEvent } from '@dayflow/core';

const monthsBefore = 1;
const monthsAfter = 3;

class CalendarEventSearch {
	events = $state<DayFlowEvent[]>([]);

	load = async () => {
		const now = new Date();
		const startDate = new Date(now.getFullYear(), now.getMonth() - monthsBefore, 1);
		const endDate = new Date(now.getFullYear(), now.getMonth() + monthsAfter + 1, 0);
		try {
			const calendarEvents = await fetchCalendarEvents(startDate, endDate, '');
			this.events = calendarEvents.map(dayFlowEventFromCalendarEvent);
		} catch {
			this.events = [];
		}
	};

	search = (query: string): CalendarSearchResult[] => searchCalendarEvents(query, this.events);
}

export const calendarEventSearch = new CalendarEventSearch();
