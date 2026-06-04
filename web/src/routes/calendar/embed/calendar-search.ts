import type { Event as DayFlowEvent } from '@dayflow/core';
import { eventStartDate } from './calendar-event-mapping';

export type CalendarSearchResult = {
	id: string;
	title: string;
	startDate: Date;
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
		.filter((event) => event.title.toLocaleLowerCase().includes(normalizedQuery))
		.sort((leftEvent, rightEvent) => eventStartDate(leftEvent).getTime() - eventStartDate(rightEvent).getTime())
		.slice(0, 8)
		.map((event) => {
			const startDate = eventStartDate(event);
			return {
				id: event.id,
				title: event.title,
				startDate,
				dateLabel: startDate.toLocaleDateString('en-US', {
					month: 'short',
					day: 'numeric',
					weekday: 'short'
				}),
				highlightParts: highlightSearchMatch(event.title, trimmedQuery)
			};
		});
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
