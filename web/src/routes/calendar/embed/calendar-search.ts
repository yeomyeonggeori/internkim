import { matchesKoreanSearch } from '$lib/korean-search';
import type { CalendarModelEvent as DayFlowEvent } from './calendar-event-model';
import { eventEndDate, eventStartDate } from './calendar-event-mapping';
import { defaultCalendarEventColor } from '../grid/calendar-grid-events';
import { calendarParticipantsFromUnknown, type CalendarParticipant } from './calendar-participants';

const weekdayLabels = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'] as const;

export type CalendarSearchResult = {
	id: string;
	title: string;
	startDate: Date;
	endDate: Date;
	isAllDay: boolean;
	color: string;
	participants: CalendarParticipant[];
	dateLabel: string;
	highlightParts: HighlightPart[];
};

export type HighlightPart = {
	text: string;
	isMatch: boolean;
};

export function searchCalendarEvents(query: string, events: DayFlowEvent[]): CalendarSearchResult[] {
	const trimmedQuery = query.trim();
	if (!trimmedQuery) return [];
	const normalizedQuery = trimmedQuery.toLocaleLowerCase();
	return events
		.filter((event) => matchesKoreanSearch(event.title, normalizedQuery))
		.sort((leftEvent, rightEvent) => eventStartDate(leftEvent).getTime() - eventStartDate(rightEvent).getTime())
		.slice(0, 8)
		.map((event) => {
			const startDate = eventStartDate(event);
			return {
				id: event.id,
				title: event.title,
				startDate,
				endDate: eventEndDate(event),
				isAllDay: event.allDay ?? false,
				color: typeof event.meta?.color === 'string' && event.meta.color ? event.meta.color : defaultCalendarEventColor,
				participants: calendarParticipantsFromUnknown(event.meta?.participants),
				dateLabel: searchDateLabel(startDate),
				highlightParts: highlightSearchMatch(event.title, trimmedQuery)
			};
		});
}

export function searchDateLabel(date: Date): string {
	const year = String(date.getFullYear());
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}.${month}.${day} ${weekdayLabels[date.getDay()]}`;
}

function highlightSearchMatch(value: string, query: string): HighlightPart[] {
	const matchIndex = value.toLocaleLowerCase().indexOf(query.toLocaleLowerCase());
	if (matchIndex < 0) return [{ text: value, isMatch: false }];
	const matchEnd = matchIndex + query.length;
	return [
		{ text: value.slice(0, matchIndex), isMatch: false },
		{ text: value.slice(matchIndex, matchEnd), isMatch: true },
		{ text: value.slice(matchEnd), isMatch: false }
	].filter((part) => part.text.length > 0);
}
